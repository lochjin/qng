package shutdown

import (
	"fmt"
	"github.com/Qitmeer/qng/common/util"
	"os"
	"path"
)

const lockFileName = "shutdown.lock"

type Tracker struct {
	filePath string
}

func (t *Tracker) Check() error {
	if !Exists(t.filePath) {
		return nil
	}
	bhbs, err := util.ReadFile(t.filePath)
	if err != nil {
		return err
	}
	if len(bhbs) <= 0 {
		return nil
	}
	max_file_size := 512
	if len(bhbs) > max_file_size*1024 { // 512 kb
		return fmt.Errorf("file size large than %d kb : %s", max_file_size, t.filePath)
	}
	err = fmt.Errorf("Illegal withdrawal at block:%s, you can cleanup your block data base by '--cleanup'.", string(bhbs))
	log.Error(err.Error())
	return err
}

func (t *Tracker) Wait(info string) error {
	outFile, err := os.OpenFile(t.filePath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}
	defer func() {
		outFile.Close()
	}()
	//
	_, err = outFile.WriteString(info)
	if err != nil {
		return err
	}
	return nil
}

func (t *Tracker) Done() error {
	if !Exists(t.filePath) {
		return nil
	}
	return os.Remove(t.filePath)
}

func NewTracker(datadir string) *Tracker {
	return &Tracker{filePath: path.Join(datadir, lockFileName)}
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		return os.IsExist(err)
	}
	return true
}
