package pitchforkui

func ExamplePfUI_Page_show() {
	// Create a fake CUI, this is an example
	cui := TestingUI()

	type form struct {
		Example string `label:"Example label" pfreq:"yes" hint:"The example input"`
		Button  string `label:"Example Button" pftype:"submit"`
		Message string // Informational message goes here (eg: "it worked!")
		Error   string // Error message goes here (eg "could not perform action because ...")
	}

	// Output the page
	type Page struct {
		*PfPage
		Opt form
	}

	p := Page{cui.Page_def(), form{"Example", "", "Example Message", "Example Error"}}
	cui.Page_show("example/example.tmpl", p)

	//
	// example/example.tmpl:
	//
	// {{template "inc/header.tmpl" .}}
	// <h2>An Example Form</h2>
	// {{ pfform .UI .Opt . true }}
	// {{template "inc/footer.tmpl" .}}
	//

	// The normal flow of the code will call cui.Flush() to finally render the page.
}
