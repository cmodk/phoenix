package app

import (
	"fmt"
)

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
		return dst.Populate()
	}

	return nil

}

func (repo *DatabaseRepository) Get(dst Entity, c Criteria) error {
	err := repo.Database.MatchOne(dst, repo.Table, c)

	if err != nil {
		return err
	}

	if HasPopulate(c) {
		return dst.Populate()
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

func (repo *DatabaseRepository) Update(dst Entity) (bool, error) {
	rows_affected, err := repo.Database.Update(dst, repo.Table)

	row_updated := false
	if rows_affected == 1 {
		row_updated = true
	}

	if rows_affected > 1 {
		return false, fmt.Errorf("More than 1 row updated, %d rows updated", rows_affected)
	}

	return row_updated, err

}

func (repo *DatabaseRepository) Delete(dst Entity) error {
	rows_affected, err := repo.Database.Delete(dst, repo.Table)

	if rows_affected == 0 {
		return fmt.Errorf("Entity not found")
	}

	if rows_affected > 1 {
		return fmt.Errorf("MORE THAN 1 ROW DELETED")
	}

	return err
}

type Entity interface {
	Populate() error
}

type EntityIsNull bool
