package pitchfork

import (
	"errors"
	"strings"
)

type PfDetail struct {
	Type        string
	DisplayName string
}

func (td *PfDetail) ToString() (out string) {
	out = td.Type + " " + td.DisplayName
	return
}

func DetailType(detail string) (out string) {
	out = detail

	idx := strings.Index(detail, " ")
	if idx != -1 {
		out = detail[:idx]
	}

	return
}

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

func detail_menu(ctx PfCtx, args []string) (err error) {
	menu := NewPfMenu([]PfMEntry{
		{"list", detail_list, 0, -1, nil, PERM_USER, "List details types"},
		{"new", detail_new, 2, 2, []string{"type_name", "description"}, PERM_SYS_ADMIN, "Add a new detail type"},
	})

	err = ctx.Menu(args, menu)
	return
}
