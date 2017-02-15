// Pitchfork db (Database layer) it primarily extends golang's database/sql to provide convience functions, automatic reconnects etc
package pitchfork

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	pq "github.com/lib/pq"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
)

// ErrNoRows can be used as a shortcut to check that no rows where returned
var ErrNoRows = sql.ErrNoRows

// DB_AndOr is used to provide SQL contruction capabilities specifying either an AND or an OR
type DB_AndOr int

// DB_OP_AND and DB_OP_OR are the two current Database Operands
const (
	DB_OP_AND DB_AndOr = iota // SQL AND
	DB_OP_OR                  // SQL OR
)

// DB_Op is used to provide a operand: LIKE, ILIKE, EQ, NE, LE, GE
type DB_Op int

const (
	DB_OP_LIKE  DB_Op = iota // LIKE match *
	DB_OP_ILIKE              // ILIKE match
	DB_OP_EQ                 // EQual match
	DB_OP_NE                 // Not Equal match
	DB_OP_LE                 // Less than or Equal match
	DB_OP_GE                 // Greater than or Equal match
)

// PfDB stores the information for a Database connection
type PfDB struct {
	sql        *sql.DB // The golang SQL object
	version    int     // The version of the system schema
	appversion int     // The version of the application schema
	username   string  // The username usd for the connection
	verbosity  bool    // Whether the database should be verbose and log all queries to stdout/syslog
	silence    bool    // Whether the database code should be mostly silent
}

//  PfQuery stores the information for a query, used primary for setup and compound instructions
type PfQuery struct {
	query   string // The SQL query
	desc    string //  Description of the query
	failok  bool   // If failure of executing this query is okay
	failstr string // What the failure message is when the query fails
}

// Tx wraps a golang SQL transaction - primarily avoiding the need to import database/sql
type Tx struct {
	*sql.Tx
}

// Rows wraps a golang SQL Rows return - this to keep the query, parameters and result together, handy for logging errors
type Rows struct {
	q    string        // The SQL Query
	p    []interface{} // The SQL Query Parameters
	rows *sql.Rows     // The returned rows
	db   *PfDB         // The database object that executed the query
}

// Row is a singular version of Rows
type Row struct {
	q   string        // The SQL Query
	p   []interface{} // The SQL Query Parameters
	row *sql.Row      // The returned row
	db  *PfDB         // The database object that executed the query
}

// Global database variable - there can only be one
var DB PfDB

// DB_Init is used to initialize the database
func DB_Init(verbosity bool) {
	DB.Init(verbosity)
}

// DB_SetAppVersion is used to inform the system what the expected Application Database schema version is
func DB_SetAppVersion(ver int) {
	DB.SetAppVersion(ver)
}

// DB_IsPQErrorConstraint checks if an error is a PostgreSQL Unique Violation
//
// "duplicate key value violates unique constraint"
func DB_IsPQErrorConstraint(err error) bool {
	/* Attempt to cast to a libpq error */
	pqerr, ok := err.(*pq.Error)

	/*
	 * From https://github.com/lib/pq/blob/master/error.go
	 * "23505": "unique_violation"
	 */
	if ok && pqerr.Code == "23505" {
		return true
	}

	return false
}

// Init is used to initialize a Database object.
//
// It is used to enable/disable verbose database messages.
func (db *PfDB) Init(verbosity bool) {
	db.sql = nil

	/* Current portal_schema_version -- must match schema.sql! */
	db.version = 21

	/* No configured App DB */
	db.appversion = -1

	/* Query Logging? */
	db.verbosity = verbosity
}

// SetAppVersion is used to indicate what application schema version is expected
func (db *PfDB) SetAppVersion(version int) {
	db.appversion = version
}

// Silence can be used to enable the silenced mode
func (db *PfDB) Silence(silence_enabled bool) {
	db.silence = silence_enabled
}

// outVerbosef can be used to verbosely output a formatted database message; code-location details are added
func (db *PfDB) outVerbosef(format string, arg ...interface{}) {
	if db.verbosity {
		outf(Where(2)+" DB."+format, arg...)
	}
}

// Errf can be used to output a formatted database error - code-location details are added
func (db *PfDB) outErrorf(format string, arg ...interface{}) {
	outf(Where(2)+" DB."+format, arg...)
}

// ToNullString can be used to easily convert a string into a sql.NullString object
func ToNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// ToNullInt64 can be used to easily convert a int64 into a sql.NullInt64 object
func ToNullInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: true}
}

// connect is used internally to cause a connection to be made for the database
func (db *PfDB) connect(dbname string, host string, port string, username string, password string) (err error) {
	/* Already connected, then don't do it again */
	if db.sql != nil {
		return nil
	}

	if dbname == "" {
		return errors.New("No database name provided")
	}

	var str string

	if host != "" {
		str += "host=" + host + " "
	}

	if port != "" {
		str += "port=" + port + " "
	}

	if username != "" {
		str += "user=" + username + " "
	}

	if password != "" {
		str += "password=" + password + " "
	}

	str += "dbname=" + dbname + " "

	/* Don't require SSL */
	str += "sslmode=" + Config.Db_ssl_mode + " "

	db.outVerbosef("connect: %s", str)

	/* "postgres" here is the driver */
	db.sql, err = sql.Open("postgres", str)

	/*
	 * If no errors, record the username we connected as
	 * Used to test if providing no context to exec() is correct
	 */
	if err != nil {
		db.username = username
	}

	return err
}

// disconnect is used internally to force close a database connection
func (db *PfDB) disconnect() {
	if db.sql != nil {
		db.sql.Close()
		db.sql = nil
	}
}

// Connect_def is used to connect to the default database.
//
// This is the standard database used by the application.
//
// This function is normally called from the Query/Exec related functions.
// though it can be called to prime the connectivity and to check
// that the database can be connected to.
func (db *PfDB) Connect_def() (err error) {
	return db.connect(Config.Db_name, Config.Db_host, Config.Db_port, Config.Db_user, Config.Db_pass)
}

// connect_pg is internally used to connect to the 'administrative' database (typically template0)
//
// This function is normally called from the database upgrade functions as the administrative
// database can be used to modify the schema of the database.
func (db *PfDB) connect_pg(dbname string) (err error) {
	db.disconnect()
	return db.connect(dbname, Config.Db_host, Config.Db_port, Config.Db_admin_user, Config.Db_admin_pass)
}

// TxBegin is used to start a SQL Transaction.
//
// After this multiple Query/Exec's can be performed till
// a TxRollBack() or TxCommit() are called which causes
// all the intermediary SQL commands to be ignored/forgotten
// or with a TxCommit finalized into the database.
//
// There should always be a matching TxRollback() or TxCommit()
// otherwise all the intermdiary queries done will never be
// actually performed and applied to the database.
//
// The transaction is local to the given Ctx.
func (db *PfDB) TxBegin(ctx PfCtx) (err error) {
	err = db.Connect_def()
	if err != nil {
		return err
	}

	// XXX: Verify that we are not already in a Tx
	// if we are in a Tx, increase a recursion counter
	// so that nested Tx's are possible

	var tx *sql.Tx
	tx, err = db.sql.Begin()

	if err != nil {
		ctx.SetTx(nil)
		db.outErrorf("TxBegin() failed: %s", err.Error())
	} else {
		ctx.SetTx(&Tx{tx})
		db.outVerbosef("TxBegin(%v)", tx)
	}

	return err
}

// TxRollback is used to abort/rollback a SQL Transaction.
//
// Called after a TxBegin when the transaction needs a rollback.
func (db *PfDB) TxRollback(ctx PfCtx) {
	tx := ctx.GetTx()
	if tx == nil {
		panic("TxRollback() Transaction was not open")
	}

	err := tx.Rollback()

	if err != nil {
		db.outErrorf("TxRollback() failed: %s", err.Error())
	} else {
		db.outVerbosef("TxRollback(%v) Ok", tx)
	}

	/* No Transaction anymore */
	ctx.SetTx(nil)
	return
}

// TxCommit is used to commit a SQL Transaction
//
// Called after a TxBegin and other SQL commands
// to indicate that the transaction needs to be commited.
func (db *PfDB) TxCommit(ctx PfCtx) (err error) {
	tx := ctx.GetTx()
	if tx == nil {
		err = errors.New("Transaction was not open")
		return
	}

	err = tx.Commit()
	ctx.SetTx(nil)

	if err != nil {
		db.outVerbosef("TxCommit() %s", err.Error())
	} else {
		db.outVerbosef("TxCommit() Ok")
	}

	return
}

// QI is used to Quote an Identifier
//
// Typically parameters should be used where possible.
//
// Unfortunately tablenames cannot be parameterized
// at which point this comes into play.
func (db *PfDB) QI(name string) string {
	return pq.QuoteIdentifier(name)
}

// IsSelect is used as a check to see if a SQL query is a
// SELECT statement and thus non modifying, primarily used
// to check whether audittxt is needed or not.
//
// This is a very simple test and primarily is used
// to protect against accidental programmer mistakes.
//
// Note that a smart programmer can bypass this, but as
// they are code-level already...
func (db *PfDB) IsSelect(query string) (ok bool) {
	if len(query) >= 6 && query[0:6] != "SELECT" {
		return false
	}

	return true
}

// audit causes a audit message to be recorded with formatting based on the given parameters.
//
// The ctx is primarily used for determining which user/group is performing the action.
// The audittxt is a short message that gets logged describing the changes being made.
// Placeholders like $1, $2, $3 can be used to reference the arguments given.
// The query is the SQL query being performed.
// The args contain zero or more arguments that are referenced from in the placeholder.
func (db *PfDB) audit(ctx PfCtx, audittxt string, query string, args ...interface{}) (err error) {
	/*
	 * No context is available when using tsetup
	 * which means we are executing as the 'postgres' user
	 */
	if ctx == nil {
		/*
		 * We do not audit changes in this situation
		 * Users with 'postgres' access can modify the tables directly
		 */
		return
	}

	/* Require an audittxt when we get here */
	if audittxt == "" {
		panic("No audittxt provided")
	}

	/* Format the Audit String */
	logmsg, aerr := db.formatQuery(audittxt, args...)
	if aerr != nil {
		db.outErrorf("DB.audit: Could not format audit string: '%s': %s", audittxt, aerr.Error())
		return
	}

	var member interface{}
	var user_name interface{}
	var tg_name interface{}
	var remote string

	member = nil
	user_name = nil
	tg_name = nil
	remote = ""

	if ctx != nil {
		if ctx.IsLoggedIn() {
			member = ctx.TheUser().GetUserName()
		}

		if ctx.HasSelectedUser() {
			user_name = ctx.SelectedUser().GetUserName()
		}

		if ctx.HasSelectedGroup() {
			tg_name = ctx.SelectedGroup().GetGroupName()
		}

		remote = ctx.GetRemote()
	}

	q := "INSERT INTO audit_history " +
		"(member, what, username, trustgroup, remote) " +
		"VALUES($1, $2, $3, $4, $5)"
	err = db.exec(ctx, true, 1, q, member, logmsg, user_name, tg_name, remote)
	/* Note: audit insertion errors are logged, not reported to the user */

	if err != nil {
		if Debug {
			db.outErrorf("exec(%s)[%v] audit error: %s", query, args, err.Error())
		}

		err = errors.New("Auditing error, please check the logs")
	}

	return
}

// Query is used to make a SQL Query. It is primarily a wrapper function that ensures the database is connected
// One or more results will be returned from this function.
//
// The 'query' argument consists of a full SQL argument, with placeholders ($1, $2, $3, etc)
// for the arguments. The 'args' passed in are zero or more arguments that match these placeholders.
// Using the placeholders ensures that no SQL-escaping can happen.
//
// Only SELECT queries should be using this function, as there is no audittxt and thus a query that modifies
// the database cannot be logged properly.
//
// Example Query:
//  SELECT column FROM table WHERE id = $1
//
// See also: Exec(), QueryRow()
func (db *PfDB) Query(query string, args ...interface{}) (trows *Rows, err error) {
	var rows *sql.Rows

	if !db.IsSelect(query) {
		/* Software mistake -- crash and burn */
		Errf("Audittxt not set; query: %q, args %#v", query, args)
		panic("Non-select queries require an audit message")
	}

	err = db.Connect_def()
	if err != nil {
		return nil, err
	}

	db.outVerbosef("QueryA: %s %#v", query, args)

	rows, err = db.sql.Query(query, args...)

	if err != nil {
		db.outErrorf("Query(%s)[%#v] error: %s", query, args, err.Error())

		/* When in debug mode, dump & exit, so we can trace it */
		if Debug {
			debug.PrintStack()
			os.Exit(-1)
		}

		err = errors.New("SQL Query failed")
	}

	return &Rows{query, args, rows, db}, err
}

// queryrow is used to query for a row providing an audittxt.
//
// This is a DB internal function.
//
// queryrow always only returns one result. This can thus be used
// when one knows there is only one result, when one limits the
// result to be only one result, or when using the RETURNING
// option of PostgreSQL to return a newly INSERTed column.
func (db *PfDB) queryrow(ctx PfCtx, audittxt string, query string, args ...interface{}) (trow *Row) {
	var row *sql.Row

	err := db.Connect_def()
	if err != nil {
		/* Does not report error */
		return nil
	}

	/* Transaction already in progress? (XXX: support nested Tx, see TxBegin) */
	local_tx := false
	if audittxt != "" && ctx != nil && ctx.GetTx() == nil {
		/* Create a local one */
		local_tx = true
		err = db.TxBegin(ctx)
		if err != nil {
			return
		}
	}

	if db.verbosity && !db.silence {
		db.outVerbosef("QueryRow: %s [%v]", query, args)
	}

	row = db.sql.QueryRow(query, args...)

	if audittxt != "" {
		err = db.audit(ctx, audittxt, query, args...)
	}

	/* Commit the Tx if we opened it */
	if local_tx {
		if err != nil {
			db.TxRollback(ctx)
		} else {
			err = db.TxCommit(ctx)
		}
	}

	return &Row{query, args, row, db}
}

// QueryRowNA queries for a row without audit (NO = No Audit).
//
// This should be used sparingly, mostly only for situations
// where auditting or other very transient changes are happening
// that do not belong in an auditlog forever.
func (db *PfDB) QueryRowNA(query string, args ...interface{}) (trow *Row) {
	return db.queryrow(nil, "", query, args...)
}

// QueryRowA queries for a row, with an Audittxt for situations where the query is an INSERT/UPDATE with RETURNING
func (db *PfDB) QueryRowA(ctx PfCtx, audittxt string, query string, args ...interface{}) (trow *Row) {

	if audittxt == "" && !db.IsSelect(query) {
		/* Software mistake -- crash and burn */
		panic("Non-select queries require an audit message")
	}

	return db.queryrow(ctx, audittxt, query, args...)
}

// QueryRow queries for a row, SELECT() only; thus no audittxt needed as nothing changes
func (db *PfDB) QueryRow(query string, args ...interface{}) (trow *Row) {
	return db.QueryRowA(nil, "", query, args...)
}

// Scan can be sued to parse the results of one of the QueryRow functions
func (rows *Rows) Scan(args ...interface{}) (err error) {
	err = rows.rows.Scan(args...)

	switch {
	case err == ErrNoRows:
		break

	case err != nil:
		rows.db.outErrorf("Rows.Scan(%s)[%v] error: %s", rows.q, rows.p, err.Error())
		break

	default:
		err = nil
		break
	}

	return err
}

// Next steps to the next row in a result set; returns false when no more rows are available
func (rows *Rows) Next() bool {
	return rows.rows.Next()
}

// Close closes the resultset; typically called from a 'defer'
func (rows *Rows) Close() {
	if rows != nil && rows.rows != nil {
		rows.rows.Close()
	}
}

// Scan causes the row to be scanned and returned
func (row *Row) Scan(args ...interface{}) (err error) {
	if row.row == nil {
		return ErrNoRows
	}

	err = row.row.Scan(args...)

	switch {
	case err == ErrNoRows:
		break

	case err != nil:
		row.db.outErrorf("Row.Scan(%s)[%v] error: %s", row.q, row.p, err.Error())
		break

	default:
		err = nil
		break
	}

	return
}

// formatQuery is an internal call that replaces arguments in the right place, used primarily for audit string creation.
func (db *PfDB) formatQuery(q string, args ...interface{}) (str string, err error) {
	str = ""

	for i := 0; i < len(q); i++ {
		/* Find a dollar */
		if q[i] != '$' {
			str += string(q[i])
			continue
		}

		/* Double $ ($$) thus skip it */
		if q[i+1] == '$' {
			str += string(q[i])
			i++
			continue
		}

		argnum_s := ""
		argnum := 0
		i++ /* Start looking at the char after '$' */
		for i < len(q) && q[i] >= '0' && q[i] <= '9' {
			argnum_s += string(q[i])
			i++
		}
		i-- /* Back up one car, it was not part of the argnum */

		/* Convert the argnum string to int */
		argnum, err = strconv.Atoi(argnum_s)

		if err != nil {
			return
		}

		if argnum == 0 {
			str = ""
			err = errors.New("Invalid argument number 0")
			return
		}

		/* Arguments start count at 1, array at 0 */
		argnum--

		if argnum > len(args) {
			str = ""
			err = errors.New("Argument " + strconv.Itoa(argnum) + " not provided")
			return
		}

		/* Replace the variable with an argument */
		str += ToString(args[argnum])
	}

	return
}

// exec is an internal function for executing a query.
//
// PfCtx contains selected User & group, the changed object matches these
func (db *PfDB) exec(ctx PfCtx, report bool, affected int64, query string, args ...interface{}) (err error) {
	err = db.Connect_def()
	if err != nil {
		return err
	}

	var res sql.Result

	if ctx != nil && ctx.GetTx() != nil {
		db.outVerbosef("exec(%s) Tx args: %v", query, args)
		res, err = ctx.GetTx().Exec(query, args...)
	} else {
		db.outVerbosef("exec(%s) args: %v", query, args)
		res, err = db.sql.Exec(query, args...)
	}

	/* When in debug mode, dump & exit, so we can trace it */
	if err != nil && Debug {
		db.outErrorf("exec(%s)[%v] error: %s", query, args, err.Error())
		debug.PrintStack()
		db.outErrorf("exec(%s) error: %s", query, err.Error())
		os.Exit(-1)
	}

	if report && err != nil {
		db.outErrorf("exec(%s)[%v] error: %s", query, args, err.Error())

		/*
		 * Callers should never show raw SQL error messages
		 * these are logged, with the above in the log.
		 *
		 * The below message should never been by a user.
		 *
		 * Callers should replace the error message with a
		 * message like "XYZ did not work" instead.
		 *
		 * To avoid accidental leakage, we'll replace the message
		 * here already.
		 */
		err = errors.New("Please check the server log for the exact error message")
	}

	/* Check the number of affected rows */
	if err == nil && affected != -1 {
		chg, _ := res.RowsAffected()
		if chg != affected {
			/* Return "no rows changed" error */
			if affected == 1 && chg == 0 {
				err = ErrNoRows
				return
			}

			/*
			 * Current this only generate a log entry, no golang error
			 *
			 * TODO: when all code has been properly tested, change this to returning an error
			 */
			db.outErrorf("exec(%s)[%#v] expected %d row(s) changed, but %d changed", query, args, affected, chg)
			return
		}
	}

	return
}

// execA is an internal function for executing a query.
//
// It handles Transactions.
func (db *PfDB) execA(ctx PfCtx, audittxt string, affected int64, query string, args ...interface{}) (err error) {
	/* Transaction already in progress? */
	local_tx := false

	if ctx != nil && ctx.GetTx() == nil {
		/* Create a local one */
		local_tx = true
		err = db.TxBegin(ctx)
		if err != nil {
			return
		}
	}

	/* Attempt to execute the query */
	err = db.exec(ctx, true, affected, query, args...)

	if err != nil {
		/* Exec failed, thus nothing to log either as nothing changed */
		if local_tx {
			db.TxRollback(ctx)
		}
		return
	}

	if audittxt != "" {
		err = db.audit(ctx, audittxt, query, args...)
	}

	/* Commit the Tx if we opened it */
	if local_tx {
		if err != nil {
			db.TxRollback(ctx)
		} else {
			err = db.TxCommit(ctx)
		}
	}

	return
}

// ExecNA is an Exec with No Audit
//
// Use sparingly, see QueryRowNA for details
func (db *PfDB) ExecNA(affected int64, query string, args ...interface{}) (err error) {
	return db.execA(nil, "", affected, query, args...)
}

// Exec with forced requirement for audit message; used for SQL queries that do not return rows.
//
// When just querying (SELECT) and thus not modifying data one can use the Query() and QueryRow() functions.
func (db *PfDB) Exec(ctx PfCtx, audittxt string, affected int64, query string, args ...interface{}) (err error) {
	if audittxt == "" {
		panic("db.Exec() given no audittxt")
	}

	return db.execA(ctx, audittxt, affected, query, args...)
}

// Increase is a shortcut function to increase the integer value of a column in a database.
//
// The field to increase is identified by the 'what' argument.
// The database table is identified using the 'table' argument.
// The column of the table identified using the 'ident' argument.
//
// A custom audittxt can be provided or otherwise the function will
// generate a audittxt that is logged alongside the changing of the value.
func (db *PfDB) Increase(ctx PfCtx, audittxt, table string, ident string, what string) (err error) {
	if audittxt == "" {
		audittxt = "Increased " + table + "." + what
	}

	q := "UPDATE " + db.QI(table) + " " +
		"SET " + db.QI(what) + " = " + db.QI(what) + " + 1 " +
		"WHERE ident = $1"
	err = db.Exec(ctx, audittxt, 1, q, ident)

	return
}

// set is an internal function for updating given fields of a table.
func (db *PfDB) set(ctx PfCtx, audittxt string, obj interface{}, table string, idents map[string]string, what string, val interface{}) (updated bool, err error) {
	var args []interface{}

	q := "UPDATE " + db.QI(table) + " " +
		"SET " + db.QI(what) + " = "
	db.Q_AddArg(&q, &args, val)

	for key, value := range idents {
		db.Q_AddWhere(&q, &args, key, "=", value, true, false, 1)
	}

	err = db.Exec(ctx, audittxt, -1, q, args...)

	if err == nil {
		updated = true
		err = StructMod(STRUCTOP_SET, obj, what, val)
	}

	return
}

// UpdateFieldMulti allows updating multiple fields, specified by the idents map in one go.
//
// The UpdateFieldMulti takes any kind of object and updates the rows identified
// by the 'idents', in the database table given with 'table', the db field named 'what'
// with value 'val'.
//
// Permissions can optionally be ignored by specifying checkperms = false.
func (db *PfDB) UpdateFieldMulti(ctx PfCtx, obj interface{}, idents map[string]string, table string, what string, val string, checkperms bool) (updated bool, err error) {
	var ftype string
	var fname string
	var fval string

	/* Not updated yet */
	updated = false

	/* Get the details of what we are updating and check if the field exists */
	ftype, fname, fval, err = StructDetails(ctx, obj, what, SD_Tags_Require)
	if err != nil {
		return
	}

	/* Normalize booleans */
	if ftype == "bool" {
		fval = NormalizeBoolean(fval)
		val = NormalizeBoolean(val)
	}

	/* Is the value still the same? */
	if fval == val {
		return
	}

	audittxt := "Update " + table + ": "
	/* build ident structure */
	for key, value := range idents {
		audittxt += key + " = " + value + " "
	}
	audittxt += " property " + fname

	/* Log the new value unless it is a password or an image */
	switch fname {
	case "password", "passwd_chat", "passwd_jabber", "image":
		break

	default:
		audittxt += " from '" + fval + "' to '" + val + "'"
		break
	}

	switch ftype {
	case "string":
		return db.set(ctx, audittxt, obj, table, idents, fname, val)

	case "int":
		var v int
		v, err = strconv.Atoi(val)
		if err != nil {
			return
		}
		return db.set(ctx, audittxt, obj, table, idents, fname, v)

	case "bool":
		v := IsTrue(val)
		return db.set(ctx, audittxt, obj, table, idents, fname, v)

	default:
		break
	}

	err = errors.New("Unknown Type: " + ftype)
	return
}

// UpdateFieldNP can be used to update a single field in a table, NP = NoPermissionsCheck.
//
// The UpdateField set of functions take any kind of object and update the row identified
// by the 'ident', in the database table given with 'table', the db field named 'what'
// with value 'val'.
func (db *PfDB) UpdateFieldNP(ctx PfCtx, obj interface{}, ident string, table string, what string, val string) (updated bool, err error) {
	idents := make(map[string]string)
	idents["ident"] = ident
	return db.UpdateFieldMulti(ctx, obj, idents, table, what, val, false)
}

// UpdateField can be used to update a single field in a table, permissions are checked
func (db *PfDB) UpdateField(ctx PfCtx, obj interface{}, ident string, table string, what string, val string) (updated bool, err error) {
	idents := make(map[string]string)
	idents["ident"] = ident
	return db.UpdateFieldMulti(ctx, obj, idents, table, what, val, true)
}

// UpdateFieldMsg can be used to update a field providing a message whether the update was successful or not
func (db *PfDB) UpdateFieldMsg(ctx PfCtx, obj interface{}, ident string, table string, what string, val string) (err error) {
	idents := make(map[string]string)
	idents["ident"] = ident
	return db.UpdateFieldMultiMsg(ctx, obj, idents, table, what, val)
}

// UpdateFieldMultiMsg is used to update one or more fields and outputting a message indicating success or not
func (db *PfDB) UpdateFieldMultiMsg(ctx PfCtx, obj interface{}, set map[string]string, table string, what string, val string) (err error) {
	var updated bool

	updated, err = db.UpdateFieldMulti(ctx, obj, set, table, what, val, true)

	if err == nil {
		/* These strings are parsed by handleForm() for counting updated fields */
		if updated {
			ctx.OutLn("Updated %s", what)
		} else {
			ctx.OutLn("Value for %s was already the requested value", what)
		}
	}

	return
}

// GetSchemaVersion returns the schema version
func (db *PfDB) GetSchemaVersion() (version int, err error) {
	q := "SELECT value " +
		"FROM schema_metadata " +
		"WHERE key = 'portal_schema_version'"
	DB.Silence(true)
	err = DB.QueryRow(q).Scan(&version)
	DB.Silence(false)
	return
}

// GetAppSchemaVersion returns the application schema version
func (db *PfDB) GetAppSchemaVersion() (version int, err error) {
	q := "SELECT value " +
		"FROM schema_metadata " +
		"WHERE key = 'app_schema_version'"
	DB.Silence(true)
	err = DB.QueryRow(q).Scan(&version)
	DB.Silence(false)
	return
}

// Check checks that our schema version is matching what we expect returning a message about the status
func (db *PfDB) Check() (msg string, err error) {
	msg = ""

	ver, err := db.GetSchemaVersion()

	if err != nil {
		err = errors.New("System Schema Version: " + err.Error())
		return
	}

	run_ver := strconv.Itoa(ver)
	req_ver := strconv.Itoa(db.version)

	msg = "Pitchfork Database schema versions: "
	msg += "running: " + run_ver + ", "
	msg += "required: " + req_ver + "\n"

	err = nil
	if ver > db.version {
		err = errors.New("Futuristic System Database schema already in place (" + run_ver + " > " + req_ver + ")")
	} else if ver < db.version {
		err = errors.New("System Database is outdated (" + run_ver + " > " + req_ver + ")")
	}

	if err != nil {
		return
	}

	/* No configured App DB version? */
	if db.appversion == -1 {
		return
	}

	appver, err := db.GetAppSchemaVersion()
	if err == ErrNoRows {
		/* No AppSchema, thus nothing to report */
		err = nil
		return
	}

	if err != nil {
		err = errors.New("App Schema Version: " + err.Error())
		return
	}

	run_ver = strconv.Itoa(appver)
	req_ver = strconv.Itoa(db.appversion)

	msg += "App Database schema versions: "
	msg += "running: " + run_ver + ", "
	msg += "required: " + req_ver + "\n"

	if appver > db.appversion {
		err = errors.New("Futuristic App Database schema already in place (" + run_ver + " > " + req_ver + ")")
	} else if appver < db.appversion {
		err = errors.New("App Database is outdated (" + run_ver + " > " + req_ver + ")")
	}

	return
}

// SizeReport returns the top num list of table sorted by descending size
func (db *PfDB) SizeReport(num int) (sizes [][]string, err error) {
	sizes = nil

	q := "SELECT relname, " +
		"pg_size_pretty(pg_total_relation_size(C.oid)) " +
		"FROM pg_class C " +
		"LEFT JOIN pg_namespace N ON (N.oid = C.relnamespace) " +
		"WHERE nspname NOT IN ('pg_catalog', 'information_schema') " +
		"AND C.relkind <> 'i' " +
		"AND nspname !~ '^pg_toast' " +
		"ORDER BY pg_total_relation_size(C.oid) DESC " +
		"LIMIT $1"

	rows, err := DB.Query(q, num)
	if err != nil {
		return
	}

	defer rows.rows.Close()

	for rows.rows.Next() {
		var s = []string{"", ""}

		err = rows.rows.Scan(&s[0], &s[1])
		if err != nil {
			sizes = nil
			return
		}

		sizes = append(sizes, s)
	}

	return
}

// QueryFix is used to quickly replace <<DB>>, <<USER>> and <<PASS>> in queries with their real values -- used primarily by setup functions
func (db *PfDB) QueryFix(q string) (f string) {
	f = q
	f = strings.Replace(f, "<<DB>>", db.QI(Config.Db_name), -1)
	f = strings.Replace(f, "<<USER>>", db.QI(Config.Db_user), -1)
	f = strings.Replace(f, "<<PASS>>", Config.Db_pass, -1)
	return f
}

/*
 * queries execute series of queries while replacing variables.
 *
 * These queries are *NOT* audit-logged.
 * Only code that should call this are DB upgrade scripts.
 */
func (db *PfDB) queries(f string, qs []PfQuery) (err error) {
	for _, o := range qs {
		q := db.QueryFix(o.query)

		fmt.Println(f + " * " + o.desc)

		err = db.exec(nil, false, -1, q)

		/* Some things are expected to go wrong */
		if err != nil {
			e := err.Error()
			f := db.QueryFix(o.failstr)

			// fmt.Println(f + " - error: " + e)
			if o.failok {
				/* Ignore the failure */
			} else if f != "" && e == f {
				/* Ignore the failure */
			} else {
				/* Fail */
				return
			}
		}
	}

	/* All okay */
	err = nil
	return
}

// Cleanup_psql connects to Postgresql and DROPS our database and user, thus preparing for re-setup -- used by setup for developers
func (db *PfDB) Cleanup_psql() (err error) {
	f := "Cleanup_psql"

	fmt.Println(f + " - Connecting to database named 'postgres'")

	/* Connect to the *postgres* database as postgres user, as we are cleaning up */
	err = DB.connect_pg(Config.Db_admin_db)
	if err != nil {
		return
	}

	var qs = []PfQuery{
		{"DROP DATABASE <<DB>>",
			"Destroying database",
			false, "pq: database <<DB>> does not exist"},

		{"DROP USER <<USER>>",
			"Destroying users",
			false, "pq: role <<USER>> does not exist"},
	}

	err = DB.queries(f, qs)
	if err != nil {
		fmt.Printf("Error: %s", err.Error())
	}

	return
}

// Fix_Perms ensures that the PostgreSQL permissions are correctly configured for our database.
//
// During updates/upgrades or due to manual intervention some of these grants might disappear.
// Fix_Perms ensures that grants are properly intact.
func (db *PfDB) Fix_Perms() (err error) {
	var qs = []PfQuery{
		/*
			TODO: Disabled, old code still needs access
				{"REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC",
					"Revoke Public Access",
					false, ""},
		*/

		{"GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO <<USER>>",
			"Grant Access (Tables)",
			false, ""},
		{"GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO <<USER>>",
			"Grant Access (Sequences)",
			false, ""},
	}

	f := "FixPerms"

	fmt.Println(f + " - Connecting to postgres database")

	/* Connect to the database as the postgres user */
	err = DB.connect_pg(Config.Db_name)
	if err != nil {
		return
	}

	err = DB.queries(f, qs)
	return
}

// Setup_psql creates the PostgeSQL specifics permissions, languag, user and database
func (db *PfDB) Setup_psql() (err error) {
	var qs = []PfQuery{
		{"CREATE LANGUAGE plpgsql",
			"Ensuring PSQL Language 'plpgsql' exists",
			true, "pq: language \"plpgsql\" already exists"},

		{"CREATE USER <<USER>> NOCREATEDB NOCREATEROLE ENCRYPTED PASSWORD '<<PASS>>'",
			"Create User",
			false, ""},

		{"CREATE DATABASE <<DB>> ENCODING = 'UTF-8' TEMPLATE template0",
			"Create Database",
			false, ""},

		{"REVOKE CONNECT ON DATABASE <<DB>> FROM PUBLIC",
			"Revoke Public Access",
			false, ""},

		{"GRANT CONNECT ON DATABASE <<DB>> TO <<USER>>",
			"Grant Connect",
			false, ""},
	}

	f := "Setup_psql"

	fmt.Println(f + " - Connecting to postgres database")

	/* Connect to the *postgres* database as postgres user, as we are going to set up our own */
	err = DB.connect_pg(Config.Db_admin_db)
	if err != nil {
		return
	}

	err = DB.queries(f, qs)
	return
}

// Setup_DB configures the database and upgrades it where needed.
func (db *PfDB) Setup_DB() (err error) {
	/* Connect to the *tool* database as the postgres user */
	err = DB.connect_pg(Config.Db_name)
	if err != nil {
		return
	}

	/* v0 actually installs the latest edition */
	err = DB.executeFile("DB_0.psql")
	if err == nil {
		/* Apply new changes */
		err = DB.Upgrade()
		if err != nil {
			return
		}
	}

	/* Check Application Database */
	err = DB.AppUpgrade()

	return
}

/*
 * executeFile can be used to * "Execute" a .psql file with SQL commands.
 *
 * These queries are *NOT* audit-logged
 * Only code that should call this are DB upgrade scripts
 */
func (db *PfDB) executeFile(schemafilename string) (err error) {
	var file *os.File

	fn := System_findfile("dbschemas/", schemafilename)
	if fn == "" {
		fmt.Printf("Could not find DB Schema %s in dbschemas of file roots\n", schemafilename)
		return
	}

	file, err = os.Open(fn)
	if err != nil {
		fmt.Printf("Executing DB Schema %s failed: %s\n", fn, err.Error())
		return
	}

	fmt.Println("Executing " + fn)

	defer file.Close()

	lineno := 0

	q := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		lineno++

		if len(line) == 0 || line[0] == '#' || (line[0] == '-' && line[1] == '-') {
			continue
		}

		q += line

		if line[len(line)-1] != ';' {
			q += " "
			continue
		}

		/* Dollar-quoted strings */
		if strings.Count(q, "$$") == 1 {
			continue
		}

		err = db.exec(nil, false, -1, q)
		if err != nil {
			fmt.Println("FAILED[" + fn + ":" + strconv.Itoa(lineno) + "]: " + q)
			return
		}

		q = ""
	}

	/* The last query */
	if q != "" {
		err = db.exec(nil, false, -1, q)
		if err != nil {
			fmt.Println("FAILED[" + fn + ":" + strconv.Itoa(lineno) + "]: " + q)
			return
		}
	}

	err = scanner.Err()
	if err != nil {
		fmt.Printf("DB File Scanner Error: %s\n", err.Error())
		return
	}

	fmt.Println("Setup DB - done")
	return
}

// upgradedb is an internal function used to upgrade the database schema.
//
// Upgrade from schema in database to latest version by executing the relevant files.
// Both system (systemdb = true) or applcation schema can be upgraded.
func (db *PfDB) upgradedb(systemdb bool, systemver int) (err error) {
	var ver int
	var name string
	var pfx string
	err = nil

	/* Connect to the *tool* database as postgres user */
	err = DB.connect_pg(Config.Db_name)
	if err != nil {
		return
	}

	if systemdb {
		ver, err = db.GetSchemaVersion()
		name = ""
		pfx = "DB_"
		systemver = db.version
	} else {
		ver, err = db.GetAppSchemaVersion()
		name = "App "
		pfx = "APP_DB_"
	}

	if err != nil {
		ver = 0
	}

	fmt.Printf("%sDatabase schema version is %d\n", name, ver)
	fmt.Printf("%sSystem schema version is %d\n", name, systemver)

	if ver == systemver {
		fmt.Println("Already at the correct version")
		return
	} else if ver > systemver {
		fmt.Printf("%sDatabase schema is newer than system knows\n", name)
		return
	}

	for ver < systemver {
		file := fmt.Sprintf("%s%d.psql", pfx, ver)
		fmt.Printf("Upgrading %sDatabase Schema using %s\n", name, file)
		err = db.executeFile(file)
		if err != nil {
			/*
			 * This will also quit looking for new versions
			 * if the upgrade file is not found
			 */
			return
		}

		var nver int

		if systemdb {
			nver, err = db.GetSchemaVersion()
		} else {
			nver, err = db.GetAppSchemaVersion()
		}

		if err != nil {
			fmt.Println("Could not Fetch Schema Edition")
			return
		}

		/* Avoid going back */
		if nver <= ver {
			err = errors.New("Version did not get upgraded")
			return
		}

		/* Never go back */
		ver = nver
	}

	return
}

// Upgrade upgrades the system database to the current required schema version
func (db *PfDB) Upgrade() (err error) {
	return db.upgradedb(true, 0)
}

// AppUpgrade upgrades the application database to the current required schema version
func (db *PfDB) AppUpgrade() (err error) {
	return db.upgradedb(false, db.appversion)
}

// Q_AddArg is part of the Simple query builder - it adds a argument to the given query string and argument list
func (db *PfDB) Q_AddArg(q *string, args *[]interface{}, arg interface{}) {
	if arg != nil {
		*args = append(*args, arg)
	}

	*q += "$" + strconv.Itoa((len(*args))) + " "
}

// Q_AddWhere is part of the Simple query builder.
//
// It adds a 'where' argument to the given query string and argument list, optionally using WHERE/AND/OR to tie it in.
//
// q = existing query string to append to
// args = existing argument array
// str = the column to match
// op = the operand to use for comparing the column
// arg = what to compare the column agains
// and = whether to 'AND' the WHERE clause
// multi = whether multiple 'AND's are concatenated
// argoffset = how many args where used before and thus not part of the 'where' clause
func (db *PfDB) Q_AddWhere(q *string, args *[]interface{}, str string, op string, arg interface{}, and bool, multi bool, argoffset int) {
	if len(*args) <= argoffset {
		*q += " WHERE "
	} else {
		if and {
			*q += " AND "
		} else {
			*q += " OR "
		}
	}

	if multi {
		*q += "("
	}

	*q += str
	*q += " " + op + " "

	db.Q_AddArg(q, args, arg)
}

// Q_AddMultiClose is used to end a previously opened multi-and/or construct.
func (db *PfDB) Q_AddMultiClose(q *string) {
	*q += ")"
}

// Q_AddWhereOpAnd is used to add a "... AND str op arg" construct.
func (db *PfDB) Q_AddWhereOpAnd(q *string, args *[]interface{}, str string, op string, arg interface{}) {
	db.Q_AddWhere(q, args, str, op, arg, true, false, 0)
}

// Q_AddWhereOpAnd is used to add a "... AND str = arg" construct.
func (db *PfDB) Q_AddWhereAnd(q *string, args *[]interface{}, str string, arg interface{}) {
	db.Q_AddWhere(q, args, str, "=", arg, true, false, 0)
}

// Q_AddWhereOr is used to add a "... OR str = arg" construct.
func (db *PfDB) Q_AddWhereOr(q *string, args *[]interface{}, str string, arg interface{}) {
	db.Q_AddWhere(q, args, str, "=", arg, false, false, 0)
}

// Q_AddWhereAndN is used to add a "... AND str = arg" construct, not adding the argument to args.
func (db *PfDB) Q_AddWhereAndN(q *string, args *[]interface{}, str string) {
	db.Q_AddWhere(q, args, str, "=", nil, true, false, 0)
}

// Q_AddWhereOrN is used to add a "... OR str = arg" construct, not adding the argument to args.
func (db *PfDB) Q_AddWhereOrN(q *string, args *[]interface{}, str string) {
	db.Q_AddWhere(q, args, str, "=", nil, false, false, 0)
}
