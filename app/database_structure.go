package app

import (
	"database/sql"
)

func (app *App) DatabaseVersion() (int, error) {

	var current_version int
	if err := app.Database.Get(&current_version, "SELECT version FROM database_version WHERE id = 1"); err != nil {
		return 0, err
	}

	return current_version, nil
}

func (app *App) CheckAndUpdateDatabase(database_structure []string) error {
	db := app.Database

	_, err := db.Exec("CREATE TABLE IF NOT EXISTS `database_version` ( `id` SERIAL NOT NULL , `version` INT NOT NULL ) ENGINE = InnoDB;")
	if err != nil {
		return err
	}

	var current_version int
	if err := db.Get(&current_version, "SELECT version FROM database_version WHERE id = 1"); err != nil {
		if err != sql.ErrNoRows {
			return err
		} else {
			//Create first entry
			_, err := db.Exec("INSERT INTO database_version(version) VALUES(0)")
			if err != nil {
				return err
			}
			current_version = 0
		}

	}

	log.Debugf("Current database version: %d", current_version)

	for i := current_version + 1; i < len(database_structure); i++ {
		log.Debugf("Executing: %s\n", database_structure[i])
		_, err := db.Exec(database_structure[i])
		if err != nil {
			return err
		}

		current_version++
		_, err = db.Exec("UPDATE database_version SET version = ? WHERE id = 1", current_version)
		if err != nil {
			panic(err)
		}
	}

	log.Debugf("Current database version: %d", current_version)

	return nil
}
