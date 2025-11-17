package src

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"time"

	"coolifymanager/src/config"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
)

func errorHandler(bot *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
	logID := config.LogChat()
	if logID == 0 {
		log.Printf("logger chat id not configured (LOG_ID missing); error: %v", err)
		return ext.DispatcherActionNoop
	}

	var updatePayload string
	if ctx.Update != nil {
		if updateBytes, marshalErr := json.MarshalIndent(ctx.Update, "", "  "); marshalErr == nil {
			updatePayload = html.EscapeString(string(updateBytes))
		} else {
			updatePayload = "failed to marshal update: " + marshalErr.Error()
		}
	} else {
		updatePayload = "no update in context"
	}

	message := fmt.Sprintf(
		"<b>🚨 Bot Error</b>\n<code>%s</code>\n\n<b>Update:</b>\n<blockquote expandable>%s</blockquote>\n<b>Time:</b> %s",
		html.EscapeString(err.Error()),
		updatePayload,
		time.Now().Format(time.RFC3339),
	)

	if _, sendErr := bot.SendMessage(logID, message, &gotgbot.SendMessageOpts{
		ParseMode:           "HTML",
		DisableNotification: true,
	}); sendErr != nil {
		log.Printf("failed to send error message to logger (%d): %v", logID, sendErr)
	}

	return ext.DispatcherActionNoop
}

var (
	startTime  = time.Now()
	Dispatcher = newDispatcher()
)

func newDispatcher() *ext.Dispatcher {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{Error: errorHandler, MaxRoutines: -1})
	dispatcher.AddHandler(handlers.NewCommand("start", startHandler))
	dispatcher.AddHandler(handlers.NewCommand("ping", pingCommandHandler))

	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("list_projects"), listProjectsHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("list_deployments"), listDeploymentsHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("list_environments"), listEnvironmentsHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("list_databases"), listDatabasesHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("project_menu:"), projectMenuHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("app_deployments:"), projectDeploymentsHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("app_envs:"), appEnvsHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("restart:"), restartHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("deploy:"), deployHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("logs:"), logsHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("status:"), statusHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("stop:"), stopHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("delete:"), deleteHandler))
	return dispatcher
}
