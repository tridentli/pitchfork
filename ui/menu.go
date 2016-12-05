package pitchforkui

import (
	"errors"
	"fmt"
	pf "trident.li/pitchfork/lib"
)

type PfUIFunc func(cui PfUI)

type PfUIMentry struct {
	URI   string
	Desc  string
	Perms pf.Perm
	Fun   PfUIFunc
	Subs  []PfUIMentry
}

type PfUIMenu struct {
	M []PfUIMentry
}

/* Functions */
func NewPfUIMenu(m []PfUIMentry) PfUIMenu {
	return PfUIMenu{M: m}
}

func NewPfUIMentry(URI string, Desc string, Perms pf.Perm, Fun PfUIFunc) PfUIMentry {
	return PfUIMentry{URI, Desc, Perms, Fun, nil}
}

/* Add a menu item (to the end of the menu) */
func (menu *PfUIMenu) Add(m ...PfUIMentry) {
	menu.M = append(menu.M, m...)
}

/* Replace a menu item */
func (menu *PfUIMenu) Replace(uri string, fun PfUIFunc) {
	for i, m := range menu.M {
		if m.URI == uri {
			menu.M[i].Fun = fun
			return
		}
	}
}

/* Remove an item from the menu */
func (menu *PfUIMenu) Remove(uri string) {
	for i, m := range menu.M {
		if m.URI == uri {
			menu.M = append(menu.M[:i], menu.M[i+1:]...)
			return
		}
	}
}

/* Filter menu items only leaving the URIs in allowedURI */
func (menu *PfUIMenu) Filter(allowedURI []string) {
	/* Reverse loop as we are removing items */
	for m := len(menu.M) - 1; m >= 0; m-- {
		found := false
		for _, a := range allowedURI {
			if menu.M[m].URI == a {
				found = true
				continue
			}
		}

		if !found {
			menu.M = append(menu.M[:m], menu.M[m+1:]...)
		}
	}

}

/* Or new permissions into it, useful to mark a menu item hidden */
func (menu *PfUIMenu) AddPerms(uri string, perms pf.Perm) {
	for i, m := range menu.M {
		if m.URI == uri {
			menu.M[i].Perms |= perms
			return
		}
	}
}

/* And new permissions into it, useful to remove permissions */
func (menu *PfUIMenu) DelPerms(uri string, perms pf.Perm) {
	for i, m := range menu.M {
		if m.URI == uri {
			menu.M[i].Perms &^= perms
			return
		}
	}
}

/* Override the permissions of a menu item, useful to change the full permission */
func (menu *PfUIMenu) SetPerms(uri string, perms pf.Perm) {
	for i, m := range menu.M {
		if m.URI == uri {
			menu.M[i].Perms = perms
			return
		}
	}
}

func (menu *PfUIMenu) ToLinkColPfx(cui PfUI, depth int, pfx string) (links PfLinkCol) {
	var err error

	if menu == nil {
		return
	}

	for _, m := range menu.M {
		var l PfLink

		if m.Desc == "" {
			continue
		}

		_, err = cui.CheckPerms("ToLinkColPfx("+m.Desc+")", m.Perms)
		if err != nil {
			continue
		}

		/* Don't show none+user links when logged in */
		if m.Perms&PERM_NONE > 0 && m.Perms&PERM_USER > 0 && cui.IsLoggedIn() {
			continue
		}

		/* Don't show hidden menus */
		if m.Perms&PERM_HIDDEN > 0 {
			continue
		}

		/* The actual URL */
		link := m.URI
		if len(link) > 0 && link[0] != '?' {
			link += "/"
		}

		/* Dig a deeper hole */
		if len(m.URI) > 0 && m.URI[0] != '/' {
			if pfx != "" {
				link = pfx + link
			} else {
				for i := 0; i < depth; i++ {
					link = "../" + link
				}
			}
		}

		/* Convert subs if there are any */
		var subs []PfLink
		if m.Subs != nil {
			m := NewPfUIMenu(m.Subs)
			mm := &m
			subs = mm.ToLinkColPfx(cui, depth+1, link).M
		}

		/* Add the link */
		l.Link = link
		l.Desc = m.Desc
		l.Subs = subs
		links.Add(l)
	}

	return
}

func (menu *PfUIMenu) ToLinkCol(cui PfUI, depth int) (links PfLinkCol) {
	return menu.ToLinkColPfx(cui, depth, "")
}

func (cui *PfUIS) SetPageMenu(menu *PfUIMenu) {
	cui.pagemenu = menu
	cui.pagemenudepth = 0
}

func (cui *PfUIS) MenuPath(menu PfUIMenu, path *[]string) {
	/* Toggle this to debug menus */
	dbg := false

	if dbg {
		cui.Dbgf("MenuPath(%v) searching menu", *path)
	}

	err := errors.New("Not Found")

	/* Append a slash if there is nothing left & redirect */
	if len(*path) == 0 {
		cui.SetRedirect(cui.fullpath+"/", StatusFound)
		return
	}

	/* The menu for this page */
	cui.SetPageMenu(&menu)

	for _, m := range menu.M {
		p := (*path)[0]

		if dbg {
			cui.Dbgf("MenuPath(%s) '%s'", p, m.URI)
		}

		if m.URI != p {
			continue
		}

		/* Check permissions */
		_, err = cui.CheckPerms("MenuPath("+p+")", m.Perms)
		if err != nil {
			err = fmt.Errorf("No %s permission for MenuPath(%s): %s", m.Perms, p, err.Error())
			/* Break out */
			break
		}

		/* Add Breadcrumb unless specifically stated not to */
		if m.Perms&PERM_NOCRUMB == 0 {
			cui.AddCrumb(m.URI, m.Desc, "")
		}

		/* One level deeper */
		if m.URI != "" && m.URI[0] != '?' {
			cui.pagemenudepth++
		}

		/* Skip this part of the path */
		*path = (*path)[1:]

		/* Make sure that there are no more specific subpaths */
		if m.Perms&PERM_NOSUBS != 0 {
			if cui.NoSubs() {
				H_NoAccess(cui)
				return
			}
		}

		/* Execute */
		m.Fun(cui)
		return
	}

	if dbg {
		cui.Dbgf("MenuPathv(%v) not found", path)
	}

	/* Not Found */
	cui.Err("Menu: " + err.Error())
	H_NoAccess(cui)
}

func (cui *PfUIS) UIMenu(menu PfUIMenu) {
	cui.UISubMenuOverride(&menu)

	cui.MenuPath(menu, &cui.path)
}
