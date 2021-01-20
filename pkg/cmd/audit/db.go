package audit

import (
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

// WriteToDb writes the provided events to a sqlite3 database.  masterName is used to
// indicate the source of the events (i.e. which master).  dbFilename is the name of
// the database to write the events to - if it does not exist, it will be created.
func WriteToDb(writer io.Writer, masterName, dbFilename string, events []*auditv1.Event) error {
	db, err := sql.Open("sqlite3", dbFilename)
	if err != nil {
		return err
	}
	defer db.Close()

	sqlStmt := `
CREATE TABLE IF NOT EXISTS audit_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  audit_id TEXT,
  master_name TEXT,
  level TEXT,
  stage TEXT,
  request_uri TEXT,
  verb TEXT,
  user_name TEXT,
  user_groups TEXT,
  impersonated_name TEXT,
  impersonated_groups TEXT,
  source_ips TEXT,
  user_agent TEXT,
  ref_resource TEXT,
  ref_namespace TEXT,
  ref_name TEXT,
  ref_apiversion TEXT,
  response_status TEXT,
  response_message TEXT,
  response_code INTEGER,
  request_received_timestamp TEXT,
  stage_timestamp TEXT,
  duration_microseconds INTEGER,
  annotations TEXT
);`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
insert into audit_events(
  audit_id,
  master_name,
  level,
  stage,
  request_uri,
  verb,
  user_name,
  user_groups,
  impersonated_name,
  impersonated_groups,
  source_ips,
  user_agent,
  ref_resource,
  ref_namespace,
  ref_name,
  ref_apiversion,
  response_status,
  response_message,
  response_code,
  request_received_timestamp,
  stage_timestamp,
  duration_microseconds,
  annotations
)
values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, event := range events {
		impersonatedName := ""
		impersonatedGroups := ""
		if event.ImpersonatedUser != nil {
			user := event.ImpersonatedUser
			impersonatedName = user.Username
			impersonatedGroups = strings.Join(user.Groups, ",")
		}
		refResource := ""
		refNamespace := ""
		refName := ""
		refAPIVersion := ""
		if event.ObjectRef != nil {
			ref := event.ObjectRef
			refResource = ref.Resource
			refNamespace = ref.Namespace
			refName = ref.Name
			refAPIVersion = ref.APIVersion
		}
		respStatus := ""
		respMessage := ""
		var respCode int32
		if event.ResponseStatus != nil {
			resp := event.ResponseStatus
			respStatus = resp.Status
			respMessage = resp.Message
			respCode = resp.Code
		}
		annotations := []string{}
		for k, v := range event.Annotations {
			annotations = append(annotations, fmt.Sprintf("%s=%s", k, v))
		}
		_, err = stmt.Exec(
			event.AuditID,
			masterName,
			event.Level,
			event.Stage,
			event.RequestURI,
			event.Verb,
			event.User.Username,
			strings.Join(event.User.Groups, ","),
			impersonatedName,
			impersonatedGroups,
			strings.Join(event.SourceIPs, ","),
			event.UserAgent,
			refResource,
			refNamespace,
			refName,
			refAPIVersion,
			respStatus,
			respMessage,
			respCode,
			event.RequestReceivedTimestamp.Format(time.RFC3339Nano),
			event.StageTimestamp.Format(time.RFC3339Nano),
			event.StageTimestamp.Sub(event.RequestReceivedTimestamp.Time).Microseconds(),
			strings.Join(annotations, ","),
		)
		if err != nil {
			fmt.Fprintf(writer, "error inserting:\nevent: %#v\nerror: %v\n", event, err)
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}

	fmt.Fprintf(writer, "wrote audit log for master %s to database %s\n", masterName, dbFilename)
	return nil
}
