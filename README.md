# Pitchfork

Pitchfork is a framework forming the basis for [Trident](https://trident.li) and other tools build upon this framework.

## Testing

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

### CLI Tests

Either make mini tests for the exact functions.
Or Call pf.Cmd() passing the various that need to be done.
Error or return body can then be checked.

### UI Tests

Pitchfork's URLTest module (```ui/urltest/```) contains the URL_Test() function that
accepts a URLTest structure that allows passing in various variables that act as the
request or as the response checks. One can do positive and negative checks with it.

Passing a set Username in the test causes a cookie to be created for that user and thus
it automatically looks like one is logged in as that user.
