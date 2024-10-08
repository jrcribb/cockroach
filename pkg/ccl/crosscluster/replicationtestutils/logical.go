// Copyright 2024 The Cockroach Authors.
//
// Licensed as a CockroachDB Enterprise file under the Cockroach Community
// License (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     https://github.com/cockroachdb/cockroach/blob/master/licenses/CCL.txt

package replicationtestutils

import (
	"context"
	"fmt"

	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/errors"
)

func CheckEmptyDLQs(ctx context.Context, db sqlutils.DBHandle, dbName string) error {
	dlqNameQuery := fmt.Sprintf("SELECT table_name FROM [SHOW TABLES FROM %s] where schema_name = 'crdb_replication'", dbName)
	rows, err := db.QueryContext(ctx, dlqNameQuery)
	if err != nil {
		return errors.Wrapf(err, "failed to query dlq table name for database %s", dbName)
	}
	defer rows.Close()

	var dlqTableName string
	var dlqRowCount int
	for rows.Next() {
		if err := rows.Scan(&dlqTableName); err != nil {
			return errors.Wrapf(err, "failed to scan dlq table name for database %s", dbName)
		}
		if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT count(*) FROM %s.crdb_replication.%s", dbName, dlqTableName)).Scan(&dlqRowCount); err != nil {
			return err
		}
		if dlqRowCount != 0 {
			return fmt.Errorf("expected DLQ to be empty, but found %d rows", dlqRowCount)
		}
	}
	if dlqTableName == "" {
		return errors.Newf("didn't find any any dlq tables in database %s", dbName)
	}
	return nil
}
