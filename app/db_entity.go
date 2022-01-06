package app

type DatabaseRepository struct {
	Table    string
	Database *Database
}

func (app *App) NewDatabaseRepository(table string) *DatabaseRepository {
	return &DatabaseRepository{
		Table:    table,
		Database: app.Database,
	}
}

func (repo *DatabaseRepository) List(dst Entity, c Criteria) error {

	err := repo.Database.Match(dst, repo.Table, c)

	if err != nil {
		return err
	}

	if HasPopulate(c) {
		dst.Populate()
	}

	return nil

}

func (repo *DatabaseRepository) Get(dst Entity, c Criteria) error {
	err := repo.Database.MatchOne(dst, repo.Table, c)

	if err != nil {
		return err
	}

	if HasPopulate(c) {
		dst.Populate()
	}

	return nil

}

func (repo *DatabaseRepository) Create(dst Entity) error {
	err := repo.Database.Insert(dst, repo.Table)

	if err != nil {
		return err
	}

	return nil

}

type Entity interface {
	Populate() error
}
