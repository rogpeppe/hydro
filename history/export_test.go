package history

func DBRelays(db *DB) [][]Event {
	return db.relays
}
