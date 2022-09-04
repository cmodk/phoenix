package auth

import (
	"context"
	"crypto/rsa"
	"database/sql"
	//	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/cmodk/go-simplehttp"
	pa "github.com/cmodk/phoenix/app"

	"github.com/golang-jwt/jwt/v4"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
)

type Auth struct {
	App *pa.App
}

func NewMiddleware(app *pa.App) *Auth {
	return &Auth{
		App: app,
	}
}

func (a Auth) ServeHTTP(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	auth_header := r.Header["Authorization"]

	if len(auth_header) > 0 && len(auth_header[0]) > 7 {
		var err error
		var user User
		var email string

		bearer := auth_header[0][7:]

		if len(r.Header["X-Grafana-User"]) > 0 {
			//From grafana, fetch the user email
			email = r.Header["X-Grafana-User"][0]

			var user User
			if err := a.App.Database.Get(&user, "SELECT id,email,auth_type FROM users WHERE email = ?", email); err != nil {
				if err == sql.ErrNoRows {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
				} else {
					a.App.HttpInternalError(w, err)
				}

				return
			}

			if user.AuthType == "google" && a.CheckGoogleAccessToken(bearer, email) {
				a.App.Logger.Printf("Found google user: %s\n", email)

				ctx := context.WithValue(context.Background(), "user", user)

				next(w, r.WithContext(ctx))
				return

			}

			if user.AuthType == "azure" && a.CheckAzureAccessToken(bearer, email) {
				a.App.Logger.Debugf("Found azure user: %s\n", email)

				ctx := context.WithValue(context.Background(), "user", user)

				next(w, r.WithContext(ctx))
				return
			}

			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		} else {

			//Last effort, check if it is a api key
			user, err = a.CheckAccessKey(bearer)
			if err != nil {
				a.App.Logger.WithField("error", err).Errorf("Bearer %s does not match any user or is expired", bearer)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			} else {
				ctx := context.WithValue(context.Background(), "user", user)
				next(w, r.WithContext(ctx))
				return
			}
		}

	}

	next(w, r)

}

type GoogleAccessToken struct {
	Azp           string `json:"azp"`
	Aud           string `json:"aud"`
	Sub           string `json:"sub"`
	Scope         string `json:"scope"`
	Exp           string `json:"exp"`
	ExpiresIn     string `json:"expires_in"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	AccessType    string `json:"access_type"`

	Error            string `json:"error"`
	ErrorDescription string `json:"error"`
}

func (a Auth) CheckGoogleAccessToken(bearer string, email string) bool {
	google_auth := simplehttp.New("https://oauth2.googleapis.com/", a.App.Logger)

	data, err := google_auth.Get(fmt.Sprintf("/tokeninfo?access_token=%s", bearer))
	if err != nil {
		a.App.Logger.WithField("error", err).Error("Error getting google token")
		return false
	}

	var token GoogleAccessToken

	if err := json.Unmarshal([]byte(data), &token); err != nil {
		a.App.Logger.WithField("error", err).Error("Error unmarshalling google token")
		return false
	}

	if token.Error != "" {
		a.App.Logger.Errorf("Google auth check returned error: %s -> %s", token.Error, token.ErrorDescription)
		return false
	}

	if err := token.Validate(email); err != nil {
		a.App.Logger.WithField("error", err).Errorf("Could not validate google token")
		return false
	}

	return true

}

type AzureIdToken struct {
	Email string `json:"email"`
}

func (a Auth) CheckAzureAccessToken(bearer string, email string) bool {
	azure_auth := simplehttp.New("https://graph.microsoft.com/oidc", a.App.Logger)

	azure_auth.SetBearerAuth(bearer)

	jwtB64 := bearer

	keySet, err := jwk.Fetch(context.Background(), "https://login.microsoftonline.com/"+a.App.Config.Azure.Tenant+"/discovery/v2.0/keys")

	token, err := jwt.Parse(jwtB64, func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != jwa.RS256.String() {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("kid header not found")
		}

		keys, ok := keySet.LookupKeyID(kid)
		if !ok {
			return nil, fmt.Errorf("key %v not found", kid)
		}

		publickey := &rsa.PublicKey{}
		err = keys.Raw(publickey)
		if err != nil {
			return nil, fmt.Errorf("could not parse pubkey")
		}

		return publickey, nil
	})

	// Check if the token is valid.
	if !token.Valid {
		a.App.Logger.Errorf("The token is not valid.")
		return false
	}
	a.App.Logger.Debugf("The token is valid.")

	return token.Valid

}

func (a Auth) CheckAccessKey(bearer string) (User, error) {
	key := ApiKey{}

	err := a.App.Database.Get(&key, "SELECT * FROM api_keys WHERE token = ?", bearer)
	if err != nil {
		return User{}, err
	}

	if time.Now().After(key.ExpirationTime) {
		a.App.Logger.Error("API key expired")
		return User{}, fmt.Errorf("API key expired")
	}

	var user User
	if err := a.App.Database.Get(&user, "SELECT id,email,auth_type,last_login FROM users WHERE id=?", key.UserId); err != nil {
		return User{}, err
	}

	return user, nil

}

func (t *GoogleAccessToken) Validate(email string) error {

	if t.Email != email {
		return fmt.Errorf("Email mismatch for google token %s != %s", t.Email, email)
	}

	exp, err := strconv.ParseInt(t.Exp, 10, 64)
	if err != nil {
		return fmt.Errorf("Could not parse expiration time: %s: %s", t.Exp, err.Error())
	}

	expiration_time := time.Unix(exp, 0)

	if time.Now().After(expiration_time) {
		fmt.Errorf("Token expired")
	}

	return nil

}
