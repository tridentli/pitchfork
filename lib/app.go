// Pitchfork app (Application) specific configuration
package pitchfork

// AppName is the default Application Name
var AppName = "Pitchfork"

// AppVersion is the default application version (typically set by compiler options)
var AppVersion = "unconfigured"

// AppCopyright details the copyright details of the application
// Set generally by the SetAppDetails call.
var AppCopyright = ""

// AppWebsite is used for indicating the home location of the website
// Set generally by the SetAppDetails call.
var AppWebsite = ""

// SetAppDetails configures application details.
//
// The server and setup utility call this to configure these values.
func SetAppDetails(name string, ver string, copyright string, website string) {
	AppName = name
	AppVersion = ver
	AppCopyright = copyright
	AppWebsite = website
}

// AppVersionStr returns the Applicatication's version string.
func AppVersionStr() string {
	return AppVersion
}

// VersionText returns the Applications version text including copyright details.
func VersionText() string {
	t := AppName + "\n" +
		"Version: " + AppVersion + "\n"

	if AppCopyright != "" {
		t += "Copyright: " + AppCopyright + "\n"
	}

	if AppWebsite != "" {
		t += "Website: " + AppWebsite + "\n"
	}

	t += "\n" +
		"Using Trident Pitchfork\n" +
		"Copyright: (C) 2015-2017 The Trident Project\n" +
		"           Portions (C) 2015 National Cyber Forensics Training Alliance\n" +
		"Website: https://trident.li\n"

	return t
}
