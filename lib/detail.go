// Pitchfork detail manages user's details
package pitchfork

import (
	"errors"
	"strings"
)

// PfDetail contains the type and displayname of a detail
type PfDetail struct {
	Type        string
	DisplayName string
}

// ToString returns a string describing the detail
func (td *PfDetail) ToString() (out string) {
	out = td.Type + " " + td.DisplayName
	return
}

// DetailType returns the type based on a string
func DetailType(detail string) (out string) {
	out = detail

	idx := strings.Index(detail, " ")
	if idx != -1 {
		out = detail[:idx]
	}

	return
}

// DetailCheck checks if a detail is a valid detail
func DetailCheck(detail string) (err error) {
	/* Verify that detail is a valid detail */
	details, err := DetailList()
	if err != nil {
		return
	}

	for _, d := range details {
		if d.Type == detail {
			err = nil
			return
		}
	}

	err = errors.New("Invalid detail type")
	return
}

// DetailList returns a list of possible details
func DetailList() (details []PfDetail, err error) {
	q := "SELECT " +
		"type, " +
		"display_name " +
		"FROM member_detail_types " +
		"ORDER BY type"
	rows, err := DB.Query(q)

	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var detail PfDetail

		err = rows.Scan(&detail.Type, &detail.DisplayName)
		if err != nil {
			details = nil
			return
		}

		details = append(details, detail)
	}
	return
}

// detail_new creates a new detail (CLI)
func detail_new(ctx PfCtx, args []string) (err error) {
	type_name := args[0]
	type_descr := args[1]

	/* Validate name */
	type_name, err = Chk_ident("Detail Type Name", type_name)
	if err != nil {
		return
	}

	q := "INSERT INTO member_detail_types " +
		"(type, display_name) " +
		"VALUES ($1, $2)"
	err = DB.Exec(ctx,
		"Added new Member Detail Type: ($1, $2)",
		1, q,
		type_name, type_descr)
	return
}

// detail_list lists all possible details (CLI)
func detail_list(ctx PfCtx, args []string) (err error) {
	details, err := DetailList()
	if err != nil {
		return
	}

	ctx.OutLn("Detail\tDescription\n")

	for _, detail := range details {
		ctx.OutLn("%s", detail.ToString())
	}

	return
}

// detail_menu provides the CLI menu for the details (CLI)
func detail_menu(ctx PfCtx, args []string) (err error) {
	menu := NewPfMenu([]PfMEntry{
		{"list", detail_list, 0, -1, nil, PERM_USER, "List details types"},
		{"new", detail_new, 2, 2, []string{"type_name", "description"}, PERM_SYS_ADMIN, "Add a new detail type"},
	})

	err = ctx.Menu(args, menu)
	return
}
