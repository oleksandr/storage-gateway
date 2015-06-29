package main

const (
	// EnvDialURL is the name of the env variable for mongodb connection
	EnvDialURL = "STORAGE_DSN"
	// EnvBindAddr is the name of env variable for binding address
	EnvBindAddr = "STORAGE_ADDR"
	// EnvGridFSDatabase is the name of the GridFS database
	EnvGridFSDatabase = "STORAGE_GRIDFS_DB"
	// EnvGridFSPrefix is the name of the GridFS collection prefix
	EnvGridFSPrefix = "STORAGE_GRIDFS_PREFIX"
	// CtxParamsKey key to store router's params
	CtxParamsKey = "params"
)
