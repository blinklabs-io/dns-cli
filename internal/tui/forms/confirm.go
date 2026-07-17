package forms

// ConfirmState tracks a pending mutating action confirmation.
type ConfirmState struct {
	Active  bool
	Title   string
	Summary string
	Action  string // action id to run on accept
}

func (c *ConfirmState) Ask(title, summary, action string) {
	c.Active = true
	c.Title = title
	c.Summary = summary
	c.Action = action
}

func (c *ConfirmState) Clear() {
	*c = ConfirmState{}
}

func (c ConfirmState) View() string {
	if !c.Active {
		return ""
	}
	return c.Title + "\n\n" + c.Summary + "\n\ny = confirm · esc = cancel"
}
