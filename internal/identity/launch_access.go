package identity

// LaunchAccess describes the configured files and data directories whose DACL
// grants must permit the selected game identity to start. Checks are read-only.
type LaunchAccess struct {
	Executable, Directory, TempDirectory string
	RequiredFiles                        []string
}
