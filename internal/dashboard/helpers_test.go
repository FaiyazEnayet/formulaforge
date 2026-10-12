package dashboard

import (
	"testing"

	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

func mustLoad(t *testing.T, name string) *models.Sheet {
	t.Helper()
	s, err := models.LoadSheetByName(name)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
