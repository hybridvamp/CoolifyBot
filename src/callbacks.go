package src

import (
	coolifyPkg "coolifymanager/src/coolity"
	"coolifymanager/src/config"
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

const defaultPerPage = 5

func ensureDev(b *gotgbot.Bot, ctx *ext.Context) bool {
	if config.IsDev(ctx.EffectiveUser.Id) {
		return true
	}
	_, _ = ctx.CallbackQuery.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
		Text:      "🚫 You are not authorized.",
		ShowAlert: true,
	})
	return false
}

func parsePageFromCallback(data, prefix string) int {
	trimmed := strings.TrimPrefix(data, prefix)
	parts := strings.Split(strings.TrimPrefix(trimmed, ":"), ":")
	if len(parts) == 0 {
		return 1
	}
	page, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil || page < 1 {
		return 1
	}
	return page
}

func derivePage(info coolifyPkg.Pagination, fallbackCount, perPage, requestedPage int) (int, int) {
	current := info.CurrentPage
	if current < 1 {
		current = requestedPage
	}

	total := info.LastPage
	if total == 0 && info.Total > 0 && perPage > 0 {
		total = (info.Total + perPage - 1) / perPage
	}
	if total == 0 {
		if perPage > 0 && fallbackCount == perPage {
			total = current + 1
		} else {
			total = current
		}
	}
	return current, total
}

func buildPaginationRow(prefix string, currentPage, totalPages int) []gotgbot.InlineKeyboardButton {
	if currentPage < 1 {
		currentPage = 1
	}
	if totalPages < currentPage {
		totalPages = currentPage
	}

	maxPageButtons := 3
	start := currentPage - 1
	if start < 1 {
		start = 1
	}
	end := start + maxPageButtons - 1
	if end > totalPages {
		end = totalPages
		start = end - maxPageButtons + 1
		if start < 1 {
			start = 1
		}
	}

	var buttons []gotgbot.InlineKeyboardButton
	buttons = append(buttons, gotgbot.InlineKeyboardButton{
		Text:         "◀ Prev",
		CallbackData: fmt.Sprintf("%s:%d", prefix, maxInt(1, currentPage-1)),
	})

	for i := start; i <= end; i++ {
		label := fmt.Sprintf("%d", i)
		if i == currentPage {
			label = "• " + label
		}
		buttons = append(buttons, gotgbot.InlineKeyboardButton{
			Text:         label,
			CallbackData: fmt.Sprintf("%s:%d", prefix, i),
		})
	}

	next := currentPage + 1
	if totalPages > 0 {
		next = minInt(totalPages, next)
	}
	buttons = append(buttons, gotgbot.InlineKeyboardButton{
		Text:         "Next ▶",
		CallbackData: fmt.Sprintf("%s:%d", prefix, maxInt(1, next)),
	})

	return buttons
}

func listProjectsHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !ensureDev(b, ctx) {
		return nil
	}

	cb := ctx.CallbackQuery
	_, _ = cb.Answer(b, nil)

	page := parsePageFromCallback(cb.Data, "list_projects")
	result, err := config.Coolify.ListApplications(page, defaultPerPage)
	if err != nil {
		_, _, err = cb.Message.EditText(b, "❌ Failed to fetch projects: "+err.Error(), nil)
		return err
	}

	apps := result.Results()
	if len(apps) == 0 {
		_, _, err = cb.Message.EditText(b, "😶 No applications found.", nil)
		return err
	}

	currentPage, totalPages := derivePage(result.PageInfo(), len(apps), defaultPerPage, page)
	var buttons [][]gotgbot.InlineKeyboardButton
	for _, app := range apps {
		text := fmt.Sprintf("📦 %s (%s)", app.Name, app.Status)
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{
			{Text: text, CallbackData: fmt.Sprintf("project_menu:%s:%d", app.UUID, currentPage)},
		})
	}
	buttons = append(buttons, buildPaginationRow("list_projects", currentPage, totalPages))

	message := fmt.Sprintf("<b>📋 Select a project</b>\nPage %d of %d", currentPage, totalPages)
	_, _, err = cb.Message.EditText(b, message, &gotgbot.EditMessageTextOpts{
		ParseMode:   "HTML",
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons},
	})
	return err
}

func projectMenuHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.CallbackQuery
	if !ensureDev(b, ctx) {
		return nil
	}
	_, _ = cb.Answer(b, nil)

	data := strings.TrimPrefix(cb.Data, "project_menu:")
	parts := strings.Split(data, ":")

	uuid := parts[0]
	fromPage := 1
	if len(parts) > 1 {
		if p, err := strconv.Atoi(parts[1]); err == nil && p > 0 {
			fromPage = p
		}
	}

	app, err := config.Coolify.GetApplicationByUUID(uuid)
	if err != nil {
		_, _, err = cb.Message.EditText(b, "❌ Failed to load project: "+err.Error(), nil)
		return err
	}

	text := fmt.Sprintf("<b>📦 %s</b>\n🌐 %s\n📄 Status: <code>%s</code>", app.Name, app.FQDN, app.Status)
	btns := [][]gotgbot.InlineKeyboardButton{
		{{Text: "🔄 Restart", CallbackData: "restart:" + uuid}, {Text: "🚀 Deploy", CallbackData: "deploy:" + uuid}},
		{{Text: "📜 Logs", CallbackData: "logs:" + uuid}, {Text: "ℹ️ Status", CallbackData: "status:" + uuid}},
		{{Text: "🚚 Deployments", CallbackData: fmt.Sprintf("app_deployments:%s:%d", uuid, 1)}, {Text: "🌱 Envs", CallbackData: "app_envs:" + uuid}},
		{{Text: "🛑 Stop", CallbackData: "stop:" + uuid}, {Text: "❌ Delete", CallbackData: "delete:" + uuid}},
		{{Text: "🔙 Back", CallbackData: fmt.Sprintf("list_projects:%d", fromPage)}},
	}

	_, _, err = cb.Message.EditText(b, text, &gotgbot.EditMessageTextOpts{
		ParseMode:   "HTML",
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: btns},
	})
	return err
}

func projectDeploymentsHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !ensureDev(b, ctx) {
		return nil
	}
	cb := ctx.CallbackQuery
	_, _ = cb.Answer(b, nil)

	data := strings.TrimPrefix(cb.Data, "app_deployments:")
	parts := strings.Split(data, ":")
	if len(parts) == 0 {
		return nil
	}
	uuid := parts[0]
	page := 1
	if len(parts) > 1 {
		if p, err := strconv.Atoi(parts[1]); err == nil && p > 0 {
			page = p
		}
	}

	result, err := config.Coolify.ListDeploymentsByApplication(uuid, page, defaultPerPage)
	if err != nil {
		_, _, _ = cb.Message.EditText(b, "❌ Failed to fetch deployments: "+err.Error(), nil)
		return err
	}

	deployments := result.Results()
	if len(deployments) == 0 {
		_, _, err = cb.Message.EditText(b, "No deployments found for this application.", nil)
		return err
	}

	currentPage, totalPages := derivePage(result.PageInfo(), len(deployments), defaultPerPage, page)

	var sb strings.Builder
	sb.WriteString("<b>🚚 Deployments</b>\n")
	for idx, d := range deployments {
		sb.WriteString(fmt.Sprintf("%d) <code>%s</code> — %s", idx+1, d.UUID, strings.ToUpper(d.Status)))
		if d.Branch != "" {
			sb.WriteString(fmt.Sprintf(" [%s]", d.Branch))
		}
		if d.Commit != "" {
			sb.WriteString(fmt.Sprintf("\n    Commit: <code>%s</code>", d.Commit))
		}
		if d.CommitMessage != "" {
			sb.WriteString(fmt.Sprintf("\n    %s", html.EscapeString(d.CommitMessage)))
		}
		sb.WriteString("\n\n")
	}

	btns := [][]gotgbot.InlineKeyboardButton{
		buildPaginationRow("app_deployments:"+uuid, currentPage, totalPages),
		{{Text: "🔙 Back", CallbackData: "project_menu:" + uuid}},
	}

	_, _, err = cb.Message.EditText(b, sb.String(), &gotgbot.EditMessageTextOpts{
		ParseMode:   "HTML",
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: btns},
	})
	return err
}

func listDeploymentsHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !ensureDev(b, ctx) {
		return nil
	}
	cb := ctx.CallbackQuery
	_, _ = cb.Answer(b, nil)

	page := parsePageFromCallback(cb.Data, "list_deployments")
	result, err := config.Coolify.ListDeployments(page, defaultPerPage)
	if err != nil {
		_, _, _ = cb.Message.EditText(b, "❌ Failed to fetch deployments: "+err.Error(), nil)
		return err
	}

	items := result.Results()
	if len(items) == 0 {
		_, _, err = cb.Message.EditText(b, "No deployments yet.", nil)
		return err
	}

	currentPage, totalPages := derivePage(result.PageInfo(), len(items), defaultPerPage, page)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>🚚 Deployments (page %d/%d)</b>\n", currentPage, totalPages))
	for idx, d := range items {
		sb.WriteString(fmt.Sprintf("%d) <code>%s</code> — %s", idx+1, d.UUID, strings.ToUpper(d.Status)))
		if d.Application != "" {
			sb.WriteString(fmt.Sprintf("\n    App: %s", html.EscapeString(d.Application)))
		}
		if d.Branch != "" {
			sb.WriteString(fmt.Sprintf(" [%s]", d.Branch))
		}
		sb.WriteString("\n\n")
	}

	btns := [][]gotgbot.InlineKeyboardButton{
		buildPaginationRow("list_deployments", currentPage, totalPages),
		{{Text: "🔙 Back", CallbackData: "list_projects:1"}},
	}

	_, _, err = cb.Message.EditText(b, sb.String(), &gotgbot.EditMessageTextOpts{
		ParseMode:   "HTML",
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: btns},
	})
	return err
}

func listEnvironmentsHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !ensureDev(b, ctx) {
		return nil
	}
	cb := ctx.CallbackQuery
	_, _ = cb.Answer(b, nil)

	page := parsePageFromCallback(cb.Data, "list_environments")
	result, err := config.Coolify.ListEnvironments(page, defaultPerPage)
	if err != nil {
		_, _, _ = cb.Message.EditText(b, "❌ Failed to fetch environments: "+err.Error(), nil)
		return err
	}

	items := result.Results()
	if len(items) == 0 {
		_, _, err = cb.Message.EditText(b, "No environments available.", nil)
		return err
	}

	currentPage, totalPages := derivePage(result.PageInfo(), len(items), defaultPerPage, page)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>🌍 Environments (page %d/%d)</b>\n", currentPage, totalPages))
	for idx, env := range items {
		sb.WriteString(fmt.Sprintf("%d) <b>%s</b> — %s\n", idx+1, html.EscapeString(env.Name), html.EscapeString(env.Description)))
		if env.UUID != "" {
			sb.WriteString(fmt.Sprintf("UUID: <code>%s</code>\n", env.UUID))
		}
		sb.WriteString("\n")
	}

	btns := [][]gotgbot.InlineKeyboardButton{
		buildPaginationRow("list_environments", currentPage, totalPages),
		{{Text: "🔙 Back", CallbackData: "list_projects:1"}},
	}

	_, _, err = cb.Message.EditText(b, sb.String(), &gotgbot.EditMessageTextOpts{
		ParseMode:   "HTML",
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: btns},
	})
	return err
}

func listDatabasesHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if !ensureDev(b, ctx) {
		return nil
	}
	cb := ctx.CallbackQuery
	_, _ = cb.Answer(b, nil)

	page := parsePageFromCallback(cb.Data, "list_databases")
	result, err := config.Coolify.ListDatabases(page, defaultPerPage)
	if err != nil {
		_, _, _ = cb.Message.EditText(b, "❌ Failed to fetch databases: "+err.Error(), nil)
		return err
	}

	items := result.Results()
	if len(items) == 0 {
		_, _, err = cb.Message.EditText(b, "No databases available.", nil)
		return err
	}

	currentPage, totalPages := derivePage(result.PageInfo(), len(items), defaultPerPage, page)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>🗄 Databases (page %d/%d)</b>\n", currentPage, totalPages))
	for idx, db := range items {
		sb.WriteString(fmt.Sprintf("%d) <b>%s</b> — %s (%s)\n", idx+1, html.EscapeString(db.Name), db.Status, db.Type))
		if db.Host != "" {
			sb.WriteString(fmt.Sprintf("Host: <code>%s:%s</code>\n", db.Host, db.Port))
		}
		if db.UUID != "" {
			sb.WriteString(fmt.Sprintf("UUID: <code>%s</code>\n", db.UUID))
		}
		sb.WriteString("\n")
	}

	btns := [][]gotgbot.InlineKeyboardButton{
		buildPaginationRow("list_databases", currentPage, totalPages),
		{{Text: "🔙 Back", CallbackData: "list_projects:1"}},
	}

	_, _, err = cb.Message.EditText(b, sb.String(), &gotgbot.EditMessageTextOpts{
		ParseMode:   "HTML",
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: btns},
	})
	return err
}

func restartHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.CallbackQuery
	if !ensureDev(b, ctx) {
		return nil
	}
	_, _ = cb.Answer(b, nil)

	uuid := strings.TrimPrefix(cb.Data, "restart:")
	res, err := config.Coolify.RestartApplicationByUUID(uuid)
	if err != nil {
		_, _, err = cb.Message.EditText(b, "❌ Restart failed: "+err.Error(), nil)
		return err
	}
	text := fmt.Sprintf("✅ Restart queued!\nDeployment UUID: <code>%s</code>", res.DeploymentUUID)
	_, _, err = cb.Message.EditText(b, text, &gotgbot.EditMessageTextOpts{ParseMode: "HTML"})
	return err
}

func deployHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.CallbackQuery
	if !ensureDev(b, ctx) {
		return nil
	}
	_, _ = cb.Answer(b, nil)

	uuid := strings.TrimPrefix(cb.Data, "deploy:")
	res, err := config.Coolify.StartApplicationDeployment(uuid, false, false)
	if err != nil {
		_, _, err = cb.Message.EditText(b, "❌ Deploy failed: "+err.Error(), nil)
		return err
	}
	text := fmt.Sprintf("✅ Deployment queued!\nDeployment UUID: <code>%s</code>", res.DeploymentUUID)
	_, _, err = cb.Message.EditText(b, text, &gotgbot.EditMessageTextOpts{ParseMode: "HTML"})
	return err
}

func logsHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.CallbackQuery
	if !ensureDev(b, ctx) {
		return nil
	}
	_, _ = cb.Answer(b, nil)

	uuid := strings.TrimPrefix(cb.Data, "logs:")
	logs, err := config.Coolify.GetApplicationLogsByUUID(uuid)
	if err != nil {
		_, _, _ = cb.Message.EditText(b, "❌ Logs error: "+err.Error(), nil)
		return ext.EndGroups
	}

	_, _, err = cb.Message.EditText(b, "<b>📜 Logs</b>\n"+html.EscapeString(logs), &gotgbot.EditMessageTextOpts{
		ParseMode: "HTML",
	})

	return err
}

func statusHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.CallbackQuery
	if !ensureDev(b, ctx) {
		return nil
	}
	_, _ = cb.Answer(b, nil)

	uuid := strings.TrimPrefix(cb.Data, "status:")
	app, err := config.Coolify.GetApplicationByUUID(uuid)
	if err != nil {
		_, _, err = cb.Message.EditText(b, "❌ Status error: "+err.Error(), nil)
		return nil
	}

	text := fmt.Sprintf("📦 <b>%s</b>\nCurrent Status: <code>%s</code>", app.Name, app.Status)
	_, _, err = cb.Message.EditText(b, text, &gotgbot.EditMessageTextOpts{ParseMode: "HTML"})
	return err
}

func stopHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.CallbackQuery
	if !ensureDev(b, ctx) {
		return nil
	}
	_, _ = cb.Answer(b, nil)

	uuid := strings.TrimPrefix(cb.Data, "stop:")
	res, err := config.Coolify.StopApplicationByUUID(uuid)
	if err != nil {
		_, _, err = cb.Message.EditText(b, "❌ Stop failed: "+err.Error(), nil)
		return nil
	}

	_, _, err = cb.Message.EditText(b, "🛑 "+res.Message, nil)
	return err
}

func deleteHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.CallbackQuery
	if !ensureDev(b, ctx) {
		return nil
	}
	_, _ = cb.Answer(b, nil)

	uuid := strings.TrimPrefix(cb.Data, "delete:")
	err := config.Coolify.DeleteApplicationByUUID(uuid)
	if err != nil {
		_, _, err = cb.Message.EditText(b, "❌ Delete failed: "+err.Error(), nil)
		return nil
	}

	_, _, err = cb.Message.EditText(b, "✅ Application deleted successfully.", nil)
	return err
}

func appEnvsHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.CallbackQuery
	if !ensureDev(b, ctx) {
		return nil
	}
	_, _ = cb.Answer(b, nil)

	uuid := strings.TrimPrefix(cb.Data, "app_envs:")
	envs, err := config.Coolify.GetApplicationEnvsByUUID(uuid)
	if err != nil {
		_, _, _ = cb.Message.EditText(b, "❌ Failed to fetch env vars: "+err.Error(), nil)
		return err
	}

	if len(envs) == 0 {
		_, _, err = cb.Message.EditText(b, "This application has no environment variables.", nil)
		return err
	}

	var sb strings.Builder
	sb.WriteString("<b>🌱 Environment Variables</b>\n")
	limit := minInt(len(envs), 20)
	for i := 0; i < limit; i++ {
		env := envs[i]
		sb.WriteString(fmt.Sprintf("%d) <code>%s</code> (build time: %t)\n", i+1, html.EscapeString(env.Key), env.IsBuildTime))
	}
	if len(envs) > limit {
		sb.WriteString(fmt.Sprintf("\n…and %d more", len(envs)-limit))
	}

	btns := [][]gotgbot.InlineKeyboardButton{
		{{Text: "🔙 Back", CallbackData: "project_menu:" + uuid}},
	}

	_, _, err = cb.Message.EditText(b, sb.String(), &gotgbot.EditMessageTextOpts{
		ParseMode:   "HTML",
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: btns},
	})
	return err
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
