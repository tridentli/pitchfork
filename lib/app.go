package pitchfork

var AppName = "Pitchfork"
var AppVersion = "unconfigured"
var AppCopyright = ""
var AppWebsite = ""

func SetAppDetails(name string, ver string, copyright string, website string) {
	AppName = name
	AppVersion = ver
	AppCopyright = copyright
	AppWebsite = website
}

func AppVersionStr() string {
	return AppVersion
}

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
		"Copyright: (C) 2015-2016 The Trident Project\n" +
		"           Portions (C) 2015 National Cyber Forensics Training Alliance\n" +
		"Website: https://trident.li\n"

	return t
}
