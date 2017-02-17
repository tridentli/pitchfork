/*

Package pitchfork is a Golang framework for secure communication platforms.

Typically one will include the lib/ (pitchfork) and ui/ (pitchforkui) subpackages.

Website: https://trident.li/

License: Apache 2.0 (See LICENSE file)

# Pitchfork Applications

Pitchfork Applications serve a way to expose Pitchfork functionality as an application.
These applications are located in the 'cmd' directory.

Applications that use Pitchfork typically implement their own wrappers around the CLI, Setup and Serve calls in cmd/cli, cmd/setup, and cmd/serve respectively.

These application specific version can then pass in custom callbacks to be called etc.

## Server (the daemon)

The Server (cmd/server/server.go) code has a large Serve() call which loads the
configuration file (as passed or as a environment variable as defined by the application)
sets up the HTTP web server and then starts serving requests.
See cmd/server/server.go for more details.

This Serve() calls gets called from the 'server'/'daemon' utility of the application.

Typically the daemon will be exposed behind a Nginx HTTP proxy that only serves content using HTTPS.

## CLI utility

CLI (cmd/cli/cli.go) implements a CLI tool for the application that allows shell based
CLI access to the daemon.

The CLI call effectively parses the arguments and optional environment variables and uses
a, typically loopback-based, HTTP request to send the command to the daemon's HTTP server.

The request comes in as a HTTP request in the daemon (H_root() as mentioned above), but as
it is targeted at the /cli/ URL, gets handled by the h_cli() function in ui/cmd.go.
This function in turn uses the CmdOut function of the context to execute and then return
the command in a buffer, outputting it back to the HTTP caller and thus the cli tool.

The 'system batch' command can be used to run batch scripts.

## Setup utility

The setup utility (cmd/setup/setup.go) does direct database access, thus bypassing most permissions. On a normal system it is restricted to be run only by the 'root' user as the configuration
file containing the database credentials can only be accessed by users in the applications group.

The setup utility allows setting up the database, upgrading the schemas when there is a new one
but also to add an initial or new user and changing passwords of users without knowing the
original password (as it does direct database access).

The setup utility also exposes a sudo command that can be used to run CLI commands as any given user.

## Wikiexport utility

The wikiexport utility can be used to export a FosWiki wiki, archiving it in a single zipfile.
That zip file can then be transported to another host, where it can be imported into the Pitchfork Wiki system.

# Configuration file

The configuration file loaded by the server/daemon and the setup tool is a JSON file
but with comments, lines starting with a hash ('#') interleaved for clarification.

See lib/cfg.go for more details and the contents of the configuration file.

# Details

## File structure and Function naming

File names are prefixed depending on their location in the (menu) system.

Function names are prefixed depending on their location in the codebase.

For example, functions related to a User are located in {lib|ui}/user.go.
The CLI entry points and menus and background functions are named user_*(), while the UI equivalents are h_user_*().

CLI functions are marked with '(CLI)' in the comment above the function to indicate that it is a entry point directly from a menu.

The h_ or H_ prefix used for UI functions indicates that it is called from the HTTP handler.
UI helper functions, those not called directly from UI menu are not prefixed with the h_ prefix as they are not handlers.

There are a few deviations from this naming scheme, but these are primarily functions in the lib/misc.go file and as that indicates contains a miscellaneous set of functions.

A function ending in an 'f' is typically a printf-style 'formatted' function that accepts a
format along with a variable amount of arguments.

A function ending in an 'A' is typically a 'Advanced' function or a recursive function: it gets called from the function without the A but provides more arguments that are not always commonly needed.

### Shared resources

The share/ directory contains shared resources that pitchfork can use.

The file_roots configuration option allows specifying one or more of these file_roots.
When trying to open a file each file_root is searched in order and the first file found
will be used.

This allows one to override a file by placing the edition that needs to override it
in the earliest file_root.

An application will thus typically put it's own file_root first in the configuration
and order the pitchfork share directory last.

#### dbschemas

share/dbschemas/ contains the database schemas, primarily used during setup and upgrade.

Each schema is versioned and allows upgrading from that version to the next edition.
The 'portal_schema_version' key in the 'schema_metadata' table keeps track which current
version of the schema is applied.

The DB_*.psql files contain the System (Pitchfork) database schemas.

The APP_*.psql files, which are located in the application's share/dbschemas/ directory contain the Application databas schema.

test_data.psql contains testing data, that developers can use for testing.

#### pwdicts

The share/pwdicts directory contains password dictionaries.

These password dictionaries are used to detect weak passwords and reject those passwords from being used.

The files (ending in .txt), contain a password per line. Lines starting with a hash ('#') are treated as comments and ignored when the password dictionaries are loaded into Pitchfork.

#### rendered

The share/rendered directly is analogous to the webroot directory, except that it provides the resources used for wiki_export.

It contains the static files needed for visualizing a rendered page.

The complete directory is recursively copied into the target directory specified as the target for wiki_export.

The Pitchfork share/rendered directory is empty as otherwise those files would be copied too into what is an application defined set of files.

#### setup

The share/setup directory contains CLI batch files with file extension ```.cli```. These can be used to configure the system by executing the contained Pitchfork CLI commands.

The batch files are plain text files with per each line a CLI command.

Each setup filename is in the format of APP_setup_???.cli where the ??? indicates a version number.

The current version of the application setup version is tracked in the database table schema_metadata under the app_setup_version key.
It can be controlled with the ```system appsetup set``` command and retrieved with the ```system appsetup get``` command.

The CLI command ```system appsetup upgrade``` will only run the batch files that have not been run previously yet thus allowing a system to upgrade itself to the latest edition it is aware of.

The setup directory might also contain relevant files for performing a setup. For example Markdown (.md) files can be stored in the directory for puproses of using them during setup. One can then run the following command:

```
group wiki system updatef /PageName "Setup added" APP_setup_x.PageName.md
```

This will update from the file named APP_setup_x.PageName.md the /PageName path of the wiki of the system group, logging that it was "Setup added".

The ```system appsetup upgrade``` command changes the internal 'current working directory' (see man getcwd(3)) to the setup files directory so that files without a full path are loaded from there.

#### templates

The share/templates directory contains Golang Templates (https://golang.org/pkg/text/template/)
which are typically used by the UI system of Pitchfork.

All templates are loaded at the start of the application.

The structure of the share/templates

#### webroot

The share/webroot contains files to be served using the static file serving ability of Pitchfork.

The subdirectories are 'css' for CSS files, 'gfx' for images, and 'js' for Javascript files.

The favicon.ico is also located here, as it can be statically served.
robots-ok.txt and robots.txt are served depending on the Robots setting in the system preferences, the first is served when indexing by robots is allowed, the second is served when robots are denied.

The files in these directories are statically served. Any file placed in them is thus directly accessible and are not checked for permissions. These directories do not generate a listing though, thus getting an index from them is not possible.

## Context

The context (ctx on the lib/cli level, cui on the UI level), retains per-request details that various parts of the code might need, eg for knowing if and which user is logged in and what groups they belong to.

It also enables overriding of creation of NewUser, NewGroup and Menus allowing applications to extend those objects with more properties without modifying the Pitchfork code base.

### Selecting Users, Groups, Mailinglist, Email and other objects

Throughout the code various functions affect a user, a group or other objects.

These objects can be selected for active comparison using the SelectUser, SelectGroup, SelectML, SelectEmail functions in context.

The selection functions require as an argument a set of permissions that have to be fulfilled by the active user for being able to select that object.

When the object is selected it can be retrieved with an equivalent Selected* function at which point it can be further used for processing.

The major usecase of the Select is that one part of the code can select for instance a user, and another part will automatically be able to use that user for it's purposes, thus avoiding the need to pass it around as a spar parameter, which would not otherwise easily/nicely be possible with the menu functions.

One major use for the selected objects is permission tests.

# Permissions

The context retains the details (username, groups, etc) about the logged in user.

Based on those, and other, properties retained in Ctx, permission decisions can be made.

The ctx's CheckPerms() (lib/ctx.go) function can be used for checking permissions and describes all the permissions that are available.

Per example, a commonly used permission is simply 'user'

The permissions are used throughout Pitchfork.

In menu's, both CLI and UI they are used to determine if a given entry is available for the selected user in the context.

In structures, handled by struct.go, a pfset and/or pfget tag can be set to indicate the permissions for that specific field. If a context does not have access to a field, that field will not be visible to that user.

For instance:
```
type Example struct {
	Field string `label:"Field" pfget:"user" pfget:"sysadmin"`
}
```
Would mean that a user can retrieve the field, but it requires a sysadmin privilege to set it.

The core permission code is located in lib/ctx.go with CheckPerm and lib/struct.go for anything that tests structure related permissions.

Permissions in pfget/pfset tag can be specified separated by commas to specify multiple permissions that would be acceptible to satisfy the permission check.
Perm's FromString function in lib/ctx handles this conversion from textual edition of a permission to the binary Perm that is used throughout.

## SysAdmin Privilege

The sysadmin privilege is gained by having the sysadmin flag set in the user's table. This can be toggled using the CLI by executing 'user set <username> sysadmin true|false' or using the user configuration UI. Of course it requires sysadmin privileges to toggle.

User's posessing the sysadmin bit are not directly a sysadmin when they login.
They first have to call swapadmin to swap from normal user to a sysadmin.

This allows a user account to have sysadmin privileges but from login to act like a normal user, till it is chosen to elevate these priveleges.

## Permission Debugging / Tracing

The functions PDbg and PDbgf contain a code level debug switch that can be flipped from false to true to enable output of the decisions being made regarding permissions in the ctx.CheckPerms() call.

When all Pitchfork permissions have been checked CheckPerms() calls the Application Permissions check, if defined.

# Menu structures

Access to menu entries is protected by the Pitchfork permission system. If none of the permissions is satisfied, then the menu is not visible
or accessible for the given caller. PERM_HIDDEN is used to indicate that a menu entry is not visible in CLI help output or in the menu structures
in the webinterface.

The menu override functions can use AddPerms/DelPerms/Remove functions of the menu objects to change permissions, remove functions or replace functions.

The special pf.PERM_NOSUBS permission notes that only the bare URL (eg: /login/) is accepted for that menu path, any arguments (eg '/login/subpath') results in a 404.
This is primarily a defense against crawlers that attempt to try all possible combinatations of paths, while we have no unique paths under that URL.
Especially useful for a crawler that mistakingly tries to keep on recursing, eg '/login/login/login/...').

## CLI

For the CLI the root of the menu is in pitchfork:lib/mainmenu.go as defined in MainMenu. Functions can be followed from there
to find out which CLI command maps to which function. Alternatively, typically the function name will map directly to a filename.
Eg the 'system' command is to be found in pitchfork:lib/system.go.

## UI

For the UI the root of the menu is in H_root (pitchfork:ui/root.go) which matches the HTTP root (/) and is where Go net/http's sends the request the first time into Pitchfork.

The Main menu is used for URL processing and finding which function should be called for it. The submenu is only used for visual appearance.

# Adding new functionality / writing applications

Pitchfork is a CLI centric application: the actual functionality typically resides in the CLI. This is done so that the CLI gets first attention and also to make permissions to actual actions all in a central location: in the CLI.

To add new functionality it is typically advised to start with the CLI function. One can then call the CLI function from the UI.

This way the task can be performed through both the UI and the CLI.
Testing most portions of the UI can then also be automated by calling the CLI functions which perform the same function, minus the HTML rendering and HTTP Form parsing, which Go handles.

Any accidental mistakes at the UI level regarding permissions is irrelevant as the CLI is the real gatekeeper to an action to be performed.

# Translation Support

Translation is fully supported by pfform and friends, though not by the CLI menu system.

Every label from an object passed to pfform() is checked for a Translate() function and when that function is
present, it is called along with the label and the target language. The resulting string is used then as the label or hint for the given string.

# IP Tracking (IPTrk)

To avoid repeated login attempts in attempt to guess a username/password combination, the system has an
IP Tracking module (pitchfork:src/iptrk.go) that tracks the amount of failures per IP address.

User's thus can be locked out when they attempt to login too many times.

Hence, do check 'iptrk list' for the user's IP address when they are unable to login.
'iptrk remove <ip address> can be used to remove an individual IP address and 'iptrk flush' can be used to flush the table.
The UI equivalent management interface can be found under the System menu (/system/iptrk/).

# ModOpts

The Pitchfork system has three modules that have Module Options (ModOpts): messages (lib/messages.go), file (lib/file.go) and wiki (lib/wiki.go).

These module options configure the module so that it prefixes a path to the data it stores, thus allowing it to be used in multiple locations.
It also configures the URLPath so that it can be exposed in the CLI and UI interfaces in multiple places.

# Testing

We include tests using the go [testing](https://golang.org/pkg/testing/) framework.

### Running Tests

Running tests requires that two environment variables are set:
```
export PITCHFORK_TOOLNAME=trident
export PITCHFORK_CONFROOT=/Users/jeroen/git/trident/tconf/
```
These specify the name of the tool (and thus the name of the configfile)
and the directory where the config file and related files are loaded from.

One can then run the tests with:
```
make tests
```
or verbosely:
```
make vtests
```

or manually with:
```
go test trident.li/pitchfork/lib -v
go test trident.li/pitchfork/ui -v
```
One can also run tests individually by specifying a filter, eg:
```
go test trident.li/pitchfork/lib -v -run IPtrk
```
which would run only the iptrk related tests.

The argument to ```-run``` is a regexp, ```AB[CD]``` would for instance match functions named
```Test_ABC``` + ```Test_ABD```.

See also the top of the *._test.go files for the simple cut&paste variants.

## CLI Tests

Either make mini tests for the exact functions.
Or Call pf.Cmd() passing the various that need to be done.
Error or return body can then be checked.

## UI Tests

Pitchfork's URLTest module (```ui/urltest/```) contains the URL_Test() function that
accepts a URLTest structure that allows passing in various variables that act as the
request or as the response checks. One can do positive and negative checks with it.

Passing a set Username in the test causes a cookie to be created for that user and thus
it automatically looks like one is logged in as that user.

# Logging

There are various "logs" kept by Pitchfork. We discuss these in the following sections.

## nginx access.log

Logged by nginx when it is fronting the Pitchfork HTTP server.

Here all HTTP accesses are logged depending on the access_log configuration variable in nginx.

Format: Apache HTTP Log Format

## Pitchfork access.log

Contains all HTTP requests in a similar style to standard Apache HTTP Logs but also with the authenticated username and other details that Pitchfork has but that nginx does not have.

The location of this file is configured using the ```logfile``` configuration directive.

Format: JSON

## SQL userevents

Any events happening to a user account. Primarily used to note succesful logins.

Format: SQL

## SQL audit_history

Any modifications to the SQL database performed through pitchfork's Database

Format: SQL

## Syslog / Logfile / stdout

This log contains system messages. Depending on verbosity of the server it will be more or less verbose.

If one is looking for error messages produced by the system, this is the place to look.

Configuration is based on daemonmode of the server, or if ```syslog``` or ```loglocation``` are specified.

All ```-debug``` level output also goes to this channel.

Format: syslog lines (arbitrary text)

# Common abbreviations

CLI = Command Line Interface
Ctx = Context
Cui = Context for User Interface
ModOpts = Pitchfork Module Options
UI = User Interface

*/
package pitchfork
