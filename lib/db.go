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

var ErrNoRows = sql.ErrNoRows

type DB_AndOr int

const (
	DB_OP_AND DB_AndOr = iota
	DB_OP_OR
)

type DB_Op int

const (
	DB_OP_LIKE DB_Op = iota
	DB_OP_ILIKE
	DB_OP_EQ
	DB_OP_NE
	DB_OP_LE
	DB_OP_GE
)

type PfDB struct {
	sql        *sql.DB
	version    int
	appversion int
	username   string
	verbosity  bool
	silence    bool
}

type PfQuery struct {
	query   string
	desc    string
	failok  bool
	failstr string
}

type Tx struct {
	*sql.Tx
}

type Rows struct {
	q    string
	p    []interface{}
	rows *sql.Rows
	db   *PfDB
}

type Row struct {
	q   string
	p   []interface{}
	row *sql.Row
	db  *PfDB
}

/* Global database variable - there can only be one */
var DB PfDB

func DB_Init(verbosity bool) {
	DB.Init(verbosity)
}

func DB_SetAppVersion(ver int) {
	DB.SetAppVersion(ver)
}

/*
 * Check for a Unique Violation
 *
 * "duplicate key value violates unique constraint"
 */
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

func (db *PfDB) Init(verbosity bool) {
	db.sql = nil

	/* Current portal_schema_version -- must match schema.sql! */
	db.version = 22

	/* No configured App DB */
	db.appversion = -1

	/* Query Logging? */
	db.verbosity = verbosity
}

func (db *PfDB) SetAppVersion(version int) {
	db.appversion = version
}

func (db *PfDB) Silence(braaf bool) {
	db.silence = braaf
}

func (db *PfDB) Verb(message string) {
	if db.verbosity {
		OutA(Where(2) + " DB." + message)
	}
}

func (db *PfDB) Verbf(format string, arg ...interface{}) {
	if db.verbosity {
		OutA(Where(2)+" DB."+format, arg...)
	}
}

func (db *PfDB) Err(message string) {
	OutA(Where(2) + " DB." + message)
}

func (db *PfDB) Errf(format string, arg ...interface{}) {
	OutA(Where(2)+" DB."+format, arg...)
}

func ToNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func ToNullInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: true}
}

/* Connect to the database */
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

	db.Verbf("connect: %s", str)

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

func (db *PfDB) disconnect() {
	if db.sql != nil {
		db.sql.Close()
		db.sql = nil
	}
}

func (db *PfDB) Connect_def() (err error) {
	return db.connect(Config.Db_name, Config.Db_host, Config.Db_port, Config.Db_user, Config.Db_pass)
}

func (db *PfDB) connect_pg(dbname string) (err error) {
	db.disconnect()
	return db.connect(dbname, Config.Db_host, Config.Db_port, Config.Db_admin_user, Config.Db_admin_pass)
}

func (db *PfDB) TxBegin(ctx PfCtx) (err error) {
	err = db.Connect_def()
	if err != nil {
		return err
	}

	var stx *sql.Tx
	stx, err = db.sql.Begin()

	if err != nil {
		ctx.SetTx(nil)
		db.Errf("TxBegin() failed: %s", err.Error())
	} else {
		ctx.SetTx(&Tx{stx})
		db.Verb("TxBegin()")
	}

	return err
}

func (db *PfDB) TxRollback(ctx PfCtx) {
	tx := ctx.GetTx()
	if tx == nil {
		panic("TxRollback() Transaction was not open")
	}

	err := tx.Rollback()

	if err != nil {
		db.Errf("TxRollback() failed: %s", err.Error())
	} else {
		db.Verb("TxRollback() Ok")
	}

	/* No Transaction anymore */
	ctx.SetTx(nil)
	return
}

func (db *PfDB) TxCommit(ctx PfCtx) (err error) {
	tx := ctx.GetTx()
	if tx == nil {
		err = errors.New("Transaction was not open")
		return
	}

	err = tx.Commit()
	ctx.SetTx(nil)

	if err != nil {
		db.Verbf("TxCommit() %s", err.Error())
	} else {
		db.Verb("TxCommit() Ok")
	}

	return
}

func (db *PfDB) QI(name string) string {
	return pq.QuoteIdentifier(name)
}

func (db *PfDB) IsSelect(query string) (ok bool) {
	if len(query) >= 6 && query[0:6] != "SELECT" {
		return false
	}

	return true
}

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
		ctx.Errf("DB.audit: Could not format audit string: '%s': %s", audittxt, aerr.Error())
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
			db.Errf("exec(%s)[%v] audit error: %s", query, args, err.Error())
		}

		err = errors.New("Auditing error, please check the logs")
	}

	return
}

/* Wrapper functions, ensuring database is connected */
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

	db.Verbf("QueryA: %s %#v", query, args)

	rows, err = db.sql.Query(query, args...)

	if err != nil {
		db.Errf("Query(%s)[%#v] error: %s", query, args, err.Error())

		/* When in debug mode, dump & exit, so we can trace it */
		if Debug {
			debug.PrintStack()
			os.Exit(-1)
		}

		err = errors.New("SQL Query failed")
	}

	return &Rows{query, args, rows, db}, err
}

func (db *PfDB) queryrow(ctx PfCtx, audittxt string, query string, args ...interface{}) (trow *Row) {
	var row *sql.Row

	err := db.Connect_def()
	if err != nil {
		/* Does not report error */
		return nil
	}

	/* Transaction already in progress? */
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
		db.Verbf("QueryRow: %s [%v]", query, args)
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

/* Query for a Row, without Audittxt; use with care */
func (db *PfDB) QueryRowNA(query string, args ...interface{}) (trow *Row) {
	return db.queryrow(nil, "", query, args...)
}

/* Query for a Row, with an Audittxt for situations where the query is an INSERT/UPDATE with RETURNING */
func (db *PfDB) QueryRowA(ctx PfCtx, audittxt string, query string, args ...interface{}) (trow *Row) {

	if audittxt == "" && !db.IsSelect(query) {
		/* Software mistake -- crash and burn */
		panic("Non-select queries require an audit message")
	}

	return db.queryrow(ctx, audittxt, query, args...)
}

/* Query for a Row, SELECT() only; thus no audittxt needed as nothing changes */
func (db *PfDB) QueryRow(query string, args ...interface{}) (trow *Row) {
	return db.QueryRowA(nil, "", query, args...)
}

func (rows *Rows) Scan(args ...interface{}) (err error) {
	err = rows.rows.Scan(args...)

	switch {
	case err == ErrNoRows:
		break

	case err != nil:
		rows.db.Errf("Rows.Scan(%s)[%v] error: %s", rows.q, rows.p, err.Error())
		break

	default:
		err = nil
		break
	}

	return err
}

func (rows *Rows) Next() bool {
	return rows.rows.Next()
}

func (rows *Rows) Close() {
	if rows != nil && rows.rows != nil {
		rows.rows.Close()
	}
}

func (row *Row) Scan(args ...interface{}) (err error) {
	if row.row == nil {
		return ErrNoRows
	}

	err = row.row.Scan(args...)

	switch {
	case err == ErrNoRows:
		break

	case err != nil:
		row.db.Errf("Row.Scan(%s)[%v] error: %s", row.q, row.p, err.Error())
		break

	default:
		err = nil
		break
	}

	return
}

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

/* PfCtx contains selected User & group, the changed object matches these */
func (db *PfDB) exec(ctx PfCtx, report bool, affected int64, query string, args ...interface{}) (err error) {
	err = db.Connect_def()
	if err != nil {
		return err
	}

	var res sql.Result

	if ctx != nil && ctx.GetTx() != nil {
		db.Verbf("exec(%s) Tx args: %v", query, args)
		res, err = ctx.GetTx().Exec(query, args...)
	} else {
		db.Verbf("exec(%s) args: %v", query, args)
		res, err = db.sql.Exec(query, args...)
	}

	/* When in debug mode, dump & exit, so we can trace it */
	if err != nil && Debug {
		db.Errf("exec(%s)[%v] error: %s", query, args, err.Error())
		debug.PrintStack()
		db.Errf("exec(%s) error: %s", query, err.Error())
		os.Exit(-1)
	}

	if report && err != nil {
		db.Errf("exec(%s)[%v] error: %s", query, args, err.Error())

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
			db.Errf("exec(%s)[%#v] expected %d row(s) changed, but %d changed", query, args, affected, chg)
			return
		}
	}

	return
}

/* Exec that does not require audittxt */
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

func (db *PfDB) ExecNA(affected int64, query string, args ...interface{}) (err error) {
	return db.execA(nil, "", affected, query, args...)
}

/* Exec() with forced requirement for audit message */
func (db *PfDB) Exec(ctx PfCtx, audittxt string, affected int64, query string, args ...interface{}) (err error) {
	if audittxt == "" {
		panic("db.Exec() given no audittxt")
	}

	return db.execA(ctx, audittxt, affected, query, args...)
}

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

func (db *PfDB) UpdateFieldNP(ctx PfCtx, obj interface{}, ident string, table string, what string, val string) (updated bool, err error) {
	idents := make(map[string]string)
	idents["ident"] = ident
	return db.UpdateFieldMulti(ctx, obj, idents, table, what, val, false)
}

func (db *PfDB) UpdateField(ctx PfCtx, obj interface{}, ident string, table string, what string, val string) (updated bool, err error) {
	idents := make(map[string]string)
	idents["ident"] = ident
	return db.UpdateFieldMulti(ctx, obj, idents, table, what, val, true)
}

func (db *PfDB) UpdateFieldMsg(ctx PfCtx, obj interface{}, ident string, table string, what string, val string) (err error) {
	idents := make(map[string]string)
	idents["ident"] = ident
	return db.UpdateFieldMultiMsg(ctx, obj, idents, table, what, val)
}

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

func (db *PfDB) GetSchemaVersion() (version int, err error) {
	q := "SELECT value " +
		"FROM schema_metadata " +
		"WHERE key = 'portal_schema_version'"
	DB.Silence(true)
	err = DB.QueryRow(q).Scan(&version)
	DB.Silence(false)
	return
}

func (db *PfDB) GetAppSchemaVersion() (version int, err error) {
	q := "SELECT value " +
		"FROM schema_metadata " +
		"WHERE key = 'app_schema_version'"
	DB.Silence(true)
	err = DB.QueryRow(q).Scan(&version)
	DB.Silence(false)
	return
}

/* Checks that our schema version is matching what we expect */
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

func (db *PfDB) QueryFix(q string) (f string) {
	f = q
	f = strings.Replace(f, "<<DB>>", db.QI(Config.Db_name), -1)
	f = strings.Replace(f, "<<USER>>", db.QI(Config.Db_user), -1)
	f = strings.Replace(f, "<<PASS>>", Config.Db_pass, -1)
	return f
}

/*
 * Execute series of queries while replacing variables
 *
 * These queries are *NOT* audit-logged
 * Only code that should call this are DB upgrade scripts
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
 * "Execute" a .psql file with SQL commands
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

/* Upgrade from schema in database to latest version by executing the relevant files */
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

func (db *PfDB) Upgrade() (err error) {
	return db.upgradedb(true, 0)
}

func (db *PfDB) AppUpgrade() (err error) {
	return db.upgradedb(false, db.appversion)
}

/* Simple query builder */
func (db *PfDB) Q_AddArg(q *string, args *[]interface{}, arg interface{}) {
	if arg != nil {
		*args = append(*args, arg)
	}

	*q += "$" + strconv.Itoa((len(*args))) + " "
}

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

func (db *PfDB) Q_AddMultiClose(q *string) {
	*q += ")"
}

func (db *PfDB) Q_AddWhereOpAnd(q *string, args *[]interface{}, str string, op string, arg interface{}) {
	db.Q_AddWhere(q, args, str, op, arg, true, false, 0)
}

func (db *PfDB) Q_AddWhereAnd(q *string, args *[]interface{}, str string, arg interface{}) {
	db.Q_AddWhere(q, args, str, "=", arg, true, false, 0)
}

func (db *PfDB) Q_AddWhereOr(q *string, args *[]interface{}, str string, arg interface{}) {
	db.Q_AddWhere(q, args, str, "=", arg, false, false, 0)
}

func (db *PfDB) Q_AddWhereAndN(q *string, args *[]interface{}, str string) {
	db.Q_AddWhere(q, args, str, "=", nil, true, false, 0)
}

func (db *PfDB) Q_AddWhereOrN(q *string, args *[]interface{}, str string) {
	db.Q_AddWhere(q, args, str, "=", nil, false, false, 0)
}

func NI64(n int64) sql.NullInt64 {
	return sql.NullInt64{Int64: n, Valid: true}
}
