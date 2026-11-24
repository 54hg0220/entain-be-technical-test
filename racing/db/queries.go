package db

const (
	racesList = "list"
	racesGet  = "get"
)

func getRaceQueries() map[string]string {
	return map[string]string{
		racesList: `
			SELECT 
				id, 
				meeting_id, 
				name, 
				number, 
				visible, 
				advertised_start_time 
			FROM races
		`,
		// racesGet selects the same columns as racesList, in the same order,
		// so both can be read with scanRace.
		racesGet: `
			SELECT 
				id, 
				meeting_id, 
				name, 
				number, 
				visible, 
				advertised_start_time 
			FROM races
			WHERE id = ?
		`,
	}
}
