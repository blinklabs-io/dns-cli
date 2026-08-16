package prereq

import "os"

// ContractsOK reports whether path points to an existing plutus.json blueprint file.
func ContractsOK(path string) bool {
	if path == "" {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
