package db

import (
	"math/rand"
	"sort"
	"time"

	"syreclabs.com/go/faker"
)

// seedCompetitions maps each seeded sport to the competition its events are
// part of.
var seedCompetitions = map[string]string{
	"AFL":        "AFL Premiership",
	"NRL":        "NRL Telstra Premiership",
	"Soccer":     "A-League Men",
	"Basketball": "NBL",
	"Cricket":    "Big Bash League",
}

func (r *eventsRepo) seed() error {
	if _, err := r.db.Exec(`CREATE TABLE IF NOT EXISTS events (id INTEGER PRIMARY KEY, name TEXT, sport TEXT, competition TEXT, visible INTEGER, advertised_start_time DATETIME)`); err != nil {
		return err
	}

	sportNames := make([]string, 0, len(seedCompetitions))
	for sport := range seedCompetitions {
		sportNames = append(sportNames, sport)
	}
	sort.Strings(sportNames)

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	// Rollback is a no-op once the transaction has been committed.
	defer func() { _ = tx.Rollback() }()

	statement, err := tx.Prepare(`INSERT OR IGNORE INTO events(id, name, sport, competition, visible, advertised_start_time) VALUES (?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer statement.Close()

	for i := 1; i <= 100; i++ {
		sport := sportNames[rand.Intn(len(sportNames))]

		home, away := faker.Team().Name(), faker.Team().Name()
		for away == home {
			away = faker.Team().Name()
		}

		if _, err := statement.Exec(
			i,
			home+" vs "+away,
			sport,
			seedCompetitions[sport],
			faker.Number().Between(0, 1),
			// Stored as UTC so that every row shares the same offset: the
			// column holds RFC3339 text, and SQLite compares it as a string
			// when ordering.
			faker.Time().Between(time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, 2)).UTC().Format(time.RFC3339),
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}
