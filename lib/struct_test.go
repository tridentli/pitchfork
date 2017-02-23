package pitchfork

import (
	"fmt"
)

func structmenu_set_dummy(ctx PfCtx, args []string) (err error) {
	return
}

func ExampleStructMenu() {
	/* Create a fake context, this is an example */
	ctx := testingctx()

	/* The structure we want to convert into a 'set' and 'add' menu */
	type Example struct {
		ID           int      `label:"ID" pfset:"nobody" pfget:"user" hint:"The identify of this field"`
		AField       string   `label:"Field" pfset:"user" pfget:"user" hint:"A Field a user can modify"`
		AnotherField string   `label:"Another Field" pfset:"sysadmin" pfget:"user" hint:"Another Field that only a sysadmin could modify, but any user can read"`
		Foods        []string `label:"Foods" pfset:"user" pfget:"user"`
	}

	/* Make an, empty, instance of the object */
	example := &Example{}

	getmenu, err := StructMenu(ctx, []string{"id"}, example, false, structmenu_set_dummy)
	if err != nil {
		fmt.Printf("Problem generating menu: %s", err.Error())
	}
	fmt.Printf("%#v", getmenu)
	/*
	 * When linked under 'example get' results in a menu that can be used with the CLI commands:
	 * example get <id> id
	 * example get <id> afield
	 * example get <id> anotherfield
	 */

	setmenu, err := StructMenu(ctx, []string{"id"}, example, false, structmenu_set_dummy)
	if err != nil {
		fmt.Printf("Problem generating menu: %s", err.Error())
	}
	fmt.Printf("%#v", setmenu)
	/*
	 * When linked under 'example set' results in a menu that can be used with the CLI commands:
	 * example set <id> id <value>
	 * example set <id> afield <value>
	 * example set <id> anotherfield <value> -- sysadmin required
	 */

	addmenu, err := StructMenu(ctx, []string{"id"}, example, true, structmenu_set_dummy)
	if err != nil {
		fmt.Printf("Problem generating menu: %s", err.Error())
	}
	fmt.Printf("%#v", addmenu)
	/*
	 * When linked under 'example add' results in a menu that can be used with the CLI commands:
	 * example add <id> foods <value>
	 *
	 * The 'remove' menu is identical in output, but given another function will remove items.
	 */
}
