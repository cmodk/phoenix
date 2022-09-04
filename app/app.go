package app

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gocql/gocql"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/nsqio/go-nsq"
	"github.com/sirupsen/logrus"
	"github.com/urfave/negroni"

	"gopkg.in/yaml.v2"
)

var (
	app_certificate_generate    = flag.Bool("certificate-generate", false, "Generate private and public key for server, (And CA if not found)")
	app_certificate_path        = flag.String("certificate-path", "./certificates", "Search path for certificates")
	app_certificate_common_name = flag.String("common-name", "localhost", "DNS name for certificates")

	http_address         = flag.String("http-address", "0.0.0.0", "Listening address for http connections")
	http_port            = flag.Int("http-port", 4010, "Listening post for http connections")
	http_use_tls         = flag.Bool("http-use-tls", false, "Use TLS for http connections")
	http_tls_certificate = flag.String("http-tls-certificate", "server.crt", "Certificate file for tls")
	http_tls_key         = flag.String("http-tls-key", "server.key", "Private key file for tls")
	http_timeout         = flag.Int("http-timeout", 120, "Timeout for http requests in seconds")

	log *logrus.Logger
)

func CheckFlags() {

	//App might not be initialized, so create a logrus instance for the very low level operations
	if log == nil {
		log = logrus.New()
	}

	if *app_certificate_generate {
		if err := generateCertificates(); err != nil {
			panic(err)
		}
		os.Exit(0)
	}
}

type App struct {
	Environment string
	Config      *Config
	ListenAddr  string
	ListenPort  int
	Router      *mux.Router
	Http        *http.Server
	Negroni     *negroni.Negroni
	Logger      *logrus.Logger
	Database    *Database
	NsqProducer *nsq.Producer
	Cassandra   *gocql.Session
	Redis       *redis.Client

	Command *CommandBus
	Event   *EventBus

	EnableHttp bool
	UseTLS     bool

	CertificatePath string
	CACertificate   *x509.Certificate
	CAPrivateKey    *rsa.PrivateKey
}

type Database struct {
	*sqlx.DB
	Logger *logrus.Logger
}

type Criteria interface{}

//Find the field Populate (if present) and return the value
func HasPopulate(c Criteria) bool {
	v := reflect.ValueOf(c)

	populate := v.FieldByName("Populate")

	if !populate.IsValid() {
		return false
	}

	return populate.Bool()
}

func (db *Database) ParseCriteria(sb *squirrel.SelectBuilder, c Criteria) {
	c_value := reflect.ValueOf(c)
	typeOfT := c_value.Type()
	for i := 0; i < c_value.NumField(); i++ {
		f := c_value.Field(i)
		//Check if custom parsing is implemented
		v, ok := (f.Interface()).(interface {
			ParseCriteria(sb *squirrel.SelectBuilder) error
		})
		if ok {
			if err := v.ParseCriteria(sb); err != nil {
				panic(err)
			}

		} else {
			if !f.IsZero() && f.Kind() != reflect.Struct && f.Kind() != reflect.Slice {
				ft := typeOfT.Field(i)
				switch ft.Name {
				case "Limit":
					*sb = sb.Limit(uint64(f.Interface().(int)))
				case "Offset":
					*sb = sb.Offset(uint64(f.Interface().(int)))
				case "OrderBy":
					*sb = sb.OrderBy(f.Interface().(string))
				default:
					tag, ok := ft.Tag.Lookup("db")
					if ok {

						switch f.Type() {
						case reflect.TypeOf(EntityIsNull(false)):
							if f.Bool() == true {
								*sb = sb.Where(squirrel.Eq{tag: nil})
							}
						case reflect.TypeOf(EntityIntIsNot(0)):
							*sb = sb.Where(squirrel.NotEq{tag: f.Interface()})
						default:
							db.Logger.Tracef("%d: %s %s = %v -> %s\n", i,
								ft.Name, f.Type(), f.Interface(), ft.Tag.Get("db"))
							*sb = sb.Where(squirrel.Eq{tag: f.Interface()})

						}
					}
				}
			}
		}

	}
}

func (db *Database) Match(dst interface{}, table string, criteria Criteria) error {
	sb := squirrel.Select("*").From(table)
	db.ParseCriteria(&sb, criteria)

	query, args, err := sb.ToSql()
	if err != nil {
		return err
	}

	db.Logger.WithField("sql", "matchone").Tracef("Executing %s\n", query)

	return db.Select(dst, query, args...)

}

func (db *Database) MatchOne(dst interface{}, table string, criteria Criteria) error {
	sb := squirrel.Select("*").From(table)
	db.ParseCriteria(&sb, criteria)

	query, args, err := sb.ToSql()
	if err != nil {
		return err
	}

	db.Logger.WithField("sql", "matchone").Tracef("Executing %s\n", query)

	return db.Get(dst, query, args...)

}

func (db *Database) Insert(entity interface{}, table string) error {
	/*f, ok := reflect.TypeOf(entity).FieldByName("Id")
	if !ok {
		return fmt.Errorf("Entity has no Id: %s\n", getEventId(entity))
	}

	table := f.Tag.Get("table")
	if table == "" {
		return fmt.Errorf("Missing table tag for entity: %s\n", getEventId(entity))
	}*/

	ignored_fields := map[string]bool{}
	query, args, err := squirrel.Insert(table).SetMap(structToQueryMap(entity, ignored_fields)).ToSql()
	if err != nil {
		return err
	}
	db.Logger.WithField("sql", "insert").Tracef("Executing %s with args %v\n", query, args)

	result, err := db.Exec(query, args...)
	if err != nil {
		return err
	}

	last_id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	t := reflect.TypeOf(entity)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	_, ok := t.FieldByName("Id")
	if !ok {
		return nil
	}

	values := reflect.ValueOf(entity)
	if values.Kind() == reflect.Ptr {
		values = values.Elem()
	}
	id := values.FieldByName("Id")
	id.SetUint(uint64(last_id))

	return nil
}

func (db *Database) Update(entity interface{}, table string) (int64, error) {
	values := reflect.ValueOf(entity)
	if values.Kind() == reflect.Ptr {
		values = values.Elem()
	}

	//Get Id
	id := values.FieldByName("Id").Uint()

	ignored_fields := map[string]bool{
		"Id": true,
	}

	query, args, err := squirrel.Update(table).
		Where(squirrel.Eq{"id": id}).
		SetMap(structToQueryMap(entity, ignored_fields)).ToSql()
	if err != nil {
		return 0, err
	}
	db.Logger.WithField("sql", "insert").Tracef("Executing %s with args %v\n", query, args)

	result, err := db.Exec(query, args...)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()

}

func (db *Database) Delete(entity interface{}, table string) (int64, error) {
	values := reflect.ValueOf(entity)
	if values.Kind() == reflect.Ptr {
		values = values.Elem()
	}

	//Get Id
	idValue := values.FieldByName("Id")
	if idValue.IsZero() {
		return 0, fmt.Errorf("Missing id for entity: %s", table)
	}

	id := idValue.Uint()

	query, args, err := squirrel.Delete(table).Where(squirrel.Eq{"id": id}).ToSql()
	if err != nil {
		return 0, err
	}
	db.Logger.WithField("sql", "delete").Tracef("Executing %s with args %v\n", query, args)

	result, err := db.Exec(query, args...)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()

}

func structToQueryMap(s interface{}, ignore map[string]bool) map[string]interface{} {
	m := make(map[string]interface{})
	t := reflect.TypeOf(s)
	v := reflect.ValueOf(s)

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
		t = t.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		tf := t.Field(i)
		tag := tf.Tag.Get("db")
		if len(tag) == 0 {
			continue
		}

		_, ignore := ignore[tf.Name]
		if ignore {
			continue
		}

		vf := v.FieldByName(tf.Name)

		if vf.Kind() == reflect.Interface {
			//Json encode structs
			data, err := json.Marshal(vf.Interface())
			if err != nil {
				panic(err)
			}

			m[tag] = data
		} else {
			m[tag] = vf.Interface()
		}
	}

	return m
}

type Config struct {
	LogLevel   string           `yaml:"LogLevel"`
	MariaDb    *string          `yaml:"MariaDB"`
	NsqTopic   *string          `yaml:"NsqTopic"`
	NsqLookupd *string          `yaml:"NsqLookupd"`
	Nsqd       *string          `yaml:"Nsqd"`
	Redis      *string          `yaml:"Redis"`
	Cassandra  *CassandraConfig `yaml:"Cassandra"`
	EventBus   *EventBusConfig  `yaml:"EventBus"`
	Azure      *Azure           `yaml:"Azure"`
}

func New() *App {
	env := os.Getenv("PHOENIX_ENV")
	if env == "" {
		env = "dev"
	}

	log = logrus.New()

	log.Debugf("Running in environment: %s\n", env)
	config, err := LoadConfig(env)
	if err != nil {
		panic(err)
	}

	CheckFlags()

	app := &App{
		Environment:     env,
		Config:          config,
		ListenAddr:      *http_address,
		ListenPort:      *http_port,
		Router:          mux.NewRouter(),
		Logger:          log,
		EnableHttp:      false,
		UseTLS:          *http_use_tls,
		CertificatePath: *app_certificate_path,
	}

	app.Logger.Level, err = logrus.ParseLevel(config.LogLevel)
	if err != nil {
		panic(err)
	}

	log.Debugf("Using log level %s\n", app.Logger.Level.String())

	app.Command = NewCommandBus(app)
	app.Event = NewEventBus(app)

	if config.Nsqd != nil {
		nsq_config := nsq.NewConfig()
		app.NsqProducer, err = nsq.NewProducer(*config.Nsqd, nsq_config)
		if err != nil {
			panic(err)
		}
	}

	if config.Cassandra != nil {
		app.Cassandra, err = ConnectCassandra(*config.Cassandra)
		if err != nil {
			panic(err)
		}
	}

	if config.Redis != nil {
		app.Redis, err = ConnectRedis(*config.Redis)
		if err != nil {
			panic(err)
		}
	}

	if config.MariaDb != nil {
		app.ConnectMariadb()
	}

	app.Negroni = negroni.New()

	return app
}

func LoadConfig(env string) (*Config, error) {
	config_file, err := os.Open(fmt.Sprintf("config/%s.yaml", env))
	if err != nil {
		return nil, err
	}

	var config Config

	if err := yaml.NewDecoder(config_file).Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil

}

func (app *App) Run() {

	app.Negroni.UseHandler(app.Router)

	go app.Command.Listen()
	log.Debugf("Running application\n")

	app.Http = &http.Server{
		Handler:      app.Negroni,
		Addr:         fmt.Sprintf("%s:%d", app.ListenAddr, app.ListenPort),
		WriteTimeout: time.Duration(*http_timeout) * time.Second,
		ReadTimeout:  time.Duration(*http_timeout) * time.Second,
	}
	if app.EnableHttp {
		if app.UseTLS {
			log.Fatal(app.Http.ListenAndServeTLS(*http_tls_certificate, *http_tls_key))
		} else {
			app.Logger.Debug("Listening for http connections")
			log.Fatal(app.Http.ListenAndServe())
		}

	} else {
		for {
			time.Sleep(time.Second * 60)
		}
	}

}

func (app *App) LoadCertificates(load_private_key bool) error {
	log.Debugf("Loading certificates\n")
	caPublicKeyFile, err := ioutil.ReadFile(app.CertificatePath + "/server.pem")
	if err != nil {
		return err
	}
	pemBlock, _ := pem.Decode(caPublicKeyFile)
	if pemBlock == nil {
		return fmt.Errorf("pem.Decode failed")
	}
	app.CACertificate, err = x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return err
	}

	if load_private_key {
		//      private key
		caPrivateKeyFile, err := ioutil.ReadFile(app.CertificatePath + "/server.key.pem")
		if err != nil {
			panic(err)
		}
		pemBlock, _ = pem.Decode(caPrivateKeyFile)
		if pemBlock == nil {
			panic(fmt.Errorf("pem.Decode failed"))
		}
		/*
			private_key_password, ok := os.LookupEnv("CA_PRIV_KEY_PASSWORD")
			if !ok {
				panic(fmt.Errorf("Missing password for private key\n"))
			}


			der, err := x509.DecryptPEMBlock(pemBlock, []byte(private_key_password))
			if err != nil {
				panic(err)
			}*/
		privateKey, err := x509.ParsePKCS8PrivateKey(pemBlock.Bytes)
		if err != nil {
			panic(err)
		}

		app.CAPrivateKey = privateKey.(*rsa.PrivateKey)
	}

	return nil
}

func (app *App) ListenEvents() {
	app.Event.Listen()
}

func (app *App) HandleEvent(event interface{}, handler EventHandlerFunc) {
	app.Event.Handle(event, handler)
}

func (app *App) HandleCommand(cmd interface{}, handler func(interface{}) error) {
	app.Command.Handle(cmd, handler)
}

func (app *App) Use(h negroni.Handler) {
	app.Negroni.Use(h)
}

func (app *App) Get(path string, handler http.HandlerFunc) {
	app.EnableHttp = true
	app.Router.HandleFunc(path, handler).Methods("GET")
}

func (app *App) Post(path string, handler http.HandlerFunc) {
	app.EnableHttp = true
	app.Router.HandleFunc(path, handler).Methods("POST")

}

func (app *App) Put(path string, handler http.HandlerFunc) {
	app.EnableHttp = true
	app.Router.HandleFunc(path, handler).Methods("PUT")

}

func (app *App) Delete(path string, handler http.HandlerFunc) {
	app.EnableHttp = true
	app.Router.HandleFunc(path, handler).Methods("DELETE")

}

func (app *App) PathPrefix(path string, handler http.HandlerFunc) {
	app.EnableHttp = true
	app.Router.PathPrefix(path).Handler(handler)
}

func (app *App) ConnectMariadb() {
	db, err := sqlx.Connect("mysql", *app.Config.MariaDb)
	if err != nil {
		panic(err)
	}

	app.Database = &Database{db, app.Logger}

}

func (app *App) HttpInternalError(w http.ResponseWriter, err error) {
	app.HttpError(w, err, http.StatusInternalServerError)
}
func (app *App) HttpBadRequest(w http.ResponseWriter, err error) {
	app.HttpError(w, err, http.StatusBadRequest)
}

func (app *App) HttpUnauthorized(w http.ResponseWriter, err error) {
	app.HttpError(w, err, http.StatusUnauthorized)
}

func (app *App) HttpNotFound(w http.ResponseWriter, err error) {
	app.HttpError(w, err, http.StatusNotFound)
}

func (app *App) HttpError(w http.ResponseWriter, err interface{}, status int) {
	var error_string string

	switch v := err.(type) {
	case error:
		error_string = v.Error()
	case string:
		error_string = v
	case *string:
		error_string = *v
	default:
		error_string = "Unknown error"
	}

	http.Error(w, error_string, status)
}

func getEventId(event interface{}) string {
	t := reflect.TypeOf(event)
	return t.String()
}
