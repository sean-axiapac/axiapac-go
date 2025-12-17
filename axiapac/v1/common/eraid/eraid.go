package eraid

type EraID int32

const (
	UNKNOWN            EraID = 0
	Present            EraID = 1
	Draft              EraID = 2
	Deleted            EraID = 3
	Archived           EraID = 4
	Review             EraID = 5
	Template           EraID = 6
	Rejected           EraID = 7
	UncommittedChanges EraID = 11
)
