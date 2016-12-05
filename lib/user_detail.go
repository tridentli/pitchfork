package pitchfork

import (
	"time"
)

type PfUserDetail struct {
	Detail  PfDetail
	Value   string
	Entered time.Time
}

func (ud *PfUserDetail) toString() (out string) {
	out = ud.Detail.ToString()
	out += " " + ud.Value + " Entered: " + ud.Entered.Format(time.UnixDate)
	return
}

func (user *PfUserS) GetDetails() (details []PfUserDetail, err error) {
	q := "SELECT " +
		"md.type, " +
		"mdt.display_name, " +
		"md.entered, " +
		"md.value " +
		"FROM member_details md " +
		"INNER JOIN member_detail_types mdt ON md.type = mdt.type " +
		"WHERE md.member = $1"
	rows, err := DB.Query(q, user.GetUserName())
	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var user_detail PfUserDetail
		var detail PfDetail

		err = rows.Scan(&detail.Type, &detail.DisplayName, &user_detail.Entered,
			&user_detail.Value)
		if err != nil {
			return
		}

		user_detail.Detail = detail

		details = append(details, user_detail)
	}

	return
}

func user_detail_list(ctx PfCtx, args []string) (err error) {
	username := args[0]

	err = ctx.SelectUser(username, PERM_USER_SELF)
	if err != nil {
		return
	}

	user := ctx.SelectedUser()

	details, err := user.GetDetails()
	var detail PfUserDetail

	for _, detail = range details {
		ctx.OutLn(detail.toString())
	}

	return
}

func user_detail_set(ctx PfCtx, args []string) (err error) {
	username := args[0]

	err = ctx.SelectUser(username, PERM_USER_SELF)
	if err != nil {
		return
	}

	user := ctx.SelectedUser()
	detail := args[1]
	value := args[2]

	q := "INSERT INTO member_details " +
		"(member, type, value, entered) " +
		"VALUES($1, $2, $3, now())"
	err = DB.Exec(ctx,
		"Added new member_detail ($1,$2,$3)",
		1, q,
		user.GetUserName(), detail, value)
	if err != nil {
		return
	}

	return
}

func user_detail_new_type(ctx PfCtx, args []string) (err error) {
	type_name := args[0]
	type_descr := args[1]

	/* Validate name */
	type_name, err = Chk_ident("Detail Type Name", type_name)
	if err != nil {
		return
	}

	q := "INSERT INTO member_detail_types (type, display_name) " +
		"VALUES ($1, $2)"
	err = DB.Exec(ctx,
		"Added new Member Detail Type: ($1, $2)",
		1, q,
		type_name, type_descr)
	return
}

func user_detail_delete(ctx PfCtx, args []string) (err error) {
	username := args[0]

	err = ctx.SelectUser(username, PERM_USER_SELF)
	if err != nil {
		return
	}

	user := ctx.SelectedUser()
	detail := args[1]

	q := "DELETE FROM member_details " +
		"WHERE member = $1 and type = $2 "
	err = DB.Exec(ctx,
		"Delete member_detail ($1,$2)",
		1, q,
		user.GetUserName(), detail)
	if err != nil {
		return
	}

	return
}

func user_detail(ctx PfCtx, args []string) (err error) {
	menu := NewPfMenu([]PfMEntry{
		{"list", user_detail_list, 1, 1, []string{"username"}, PERM_USER, "List user details"},
		{"list_detail", detail_list, 0, 0, nil, PERM_NONE, "List all possible details"},
		{"set", user_detail_set, 3, 3, []string{"username", "detail", "value"}, PERM_USER_SELF, "Set a detail"},
		{"delete", user_detail_delete, 2, 2, []string{"username", "detail"}, PERM_USER_SELF, "Delete a detail"},
		{"new_type", user_detail_new_type, 2, 2, []string{"type_name", "description"}, PERM_SYS_ADMIN, "Add a new detail type"},
	})

	err = ctx.Menu(args, menu)
	return
}
