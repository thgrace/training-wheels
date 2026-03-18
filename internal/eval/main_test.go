package eval

import (
	"os"
	"testing"

	"github.com/thgrace/training-wheels/internal/packs"
	"github.com/thgrace/training-wheels/internal/packs/allpacks"
)

func TestMain(m *testing.M) {
	allpacks.RegisterAll(packs.DefaultRegistry())
	os.Exit(m.Run())
}
