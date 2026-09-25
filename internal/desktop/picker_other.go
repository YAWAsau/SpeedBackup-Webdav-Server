//go:build !windows

package desktop

import "fmt"

func Available() bool          { return false }
func ServiceOwnsPort(int) bool { return false }
func ChooseFolder(string) (string, bool, error) {
	return "", false, fmt.Errorf("Windows folder picker unavailable")
}
func ShowError(err error) {}
