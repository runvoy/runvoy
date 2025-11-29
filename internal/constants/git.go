package constants

// DefaultGitRef is the default Git reference to use if no reference is provided.
const DefaultGitRef = "main"

// PlaybookDirName is the name of the playbook directory in the current working directory.
const PlaybookDirName = ".runvoy"

// PlaybookFileExtensions are the valid file extensions for playbook files.
var PlaybookFileExtensions = []string{".yaml", ".yml"}
