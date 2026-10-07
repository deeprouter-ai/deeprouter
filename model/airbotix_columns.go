package model

// InitColumnNames sets the quoted names of the columns that are reserved words
// (key, group) for the database engine in use. The gateway does this itself
// when it opens its database at start-up. It is exported for tests in other
// packages that hand this package a database of their own and then go through
// code that looks a key up by its value — the gateway's key authentication and
// the billing behind it. Without it that lookup is built around an empty
// column name and fails as bad SQL.
//
// It reads common.UsingPostgreSQL, so set the engine flags first.
func InitColumnNames() {
	initCol()
}
