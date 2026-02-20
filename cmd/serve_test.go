package cmd

import "testing"

func TestServeCommandHasStartAlias(t *testing.T) {
	found := false
	for _, a := range serveCmd.Aliases {
		if a == "start" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected serve command to include start alias")
	}
}
