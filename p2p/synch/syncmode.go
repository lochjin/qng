package synch

import "fmt"

type SyncMode byte

const (
	NormalSyncMode SyncMode = 0

	LongSyncMode SyncMode = 1

	SnapSyncMode SyncMode = 2
)

var smStrings = map[SyncMode]string{
	NormalSyncMode: "NormalSyncMode",
	LongSyncMode:   "LongSyncMode",
	SnapSyncMode:   "SnapSyncMode",
}

func (sm SyncMode) String() string {
	if s, ok := smStrings[sm]; ok {
		return s
	}
	return fmt.Sprintf("Unknown SyncMode (%d)", byte(sm))
}

func (sm SyncMode) IsSnap() bool {
	return sm == SnapSyncMode
}

func (sm SyncMode) IsLong() bool {
	return sm == LongSyncMode
}
