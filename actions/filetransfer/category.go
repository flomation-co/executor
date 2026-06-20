// Package filetransfer groups the remote file-transfer actions (FTP, FTPS,
// and SFTP). The individual verbs live in subdirectories (e.g. upload/);
// this file only carries the category metadata that the manifest generator
// surfaces to the editor palette.
package filetransfer

const (
	CategoryName        = "File Transfer"
	CategoryIcon        = "server+arrow-up"
	CategoryDescription = "Move files to and from remote FTP, FTPS, and SFTP servers."
)
