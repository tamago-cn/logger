package logger

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/tamago-cn/cfg"
)

func TestLogger(t *testing.T) {
	cfg.Load("", true)
	log.Info("a")
	cfg.Save()
}
