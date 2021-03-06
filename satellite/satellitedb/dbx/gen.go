// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbx

import (
	"context"
	"fmt"

	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/private/dbutil/txutil"

	// load our cockroach sql driver for anywhere that uses this dbx.Open
	_ "storj.io/storj/private/dbutil/cockroachutil"
)

//go:generate dbx schema -d postgres -d cockroach satellitedb.dbx .
//go:generate dbx golang -d postgres -d cockroach -p dbx -t templates satellitedb.dbx .
//go:generate bash -c "( echo '//lint:file-ignore * generated file'; cat satellitedb.dbx.go ) > satellitedb.dbx.go.tmp && mv satellitedb.dbx.go{.tmp,}"
//go:generate perl -p0i -e "s,^(\\s*\"github.com/lib/pq\")\\n\\n\\1,\\1,gm" satellitedb.dbx.go

var mon = monkit.Package()

func init() {
	// catch dbx errors
	class := errs.Class("satellitedb")
	WrapErr = func(e *Error) error {
		switch e.Code {
		case ErrorCode_NoRows:
			return e.Err
		case ErrorCode_ConstraintViolation:
			return class.Wrap(&constraintError{e.Constraint, e.Err})
		}
		return class.Wrap(e)
	}
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error { return e.Err }

// Cause returns the underlying error.
func (e *Error) Cause() error { return e.Err }

type constraintError struct {
	constraint string
	err        error
}

// Unwrap returns the underlying error.
func (err *constraintError) Unwrap() error { return err.err }

// Cause returns the underlying error.
func (err *constraintError) Cause() error { return err.err }

// Error implements the error interface.
func (err *constraintError) Error() string {
	return fmt.Sprintf("violates constraint %q: %v", err.constraint, err.err)
}

// WithTx wraps DB code in a transaction
func (db *DB) WithTx(ctx context.Context, fn func(context.Context, *Tx) error) (err error) {
	tx, err := db.Open(ctx)
	if err != nil {
		return err
	}
	return txutil.ExecuteInTx(ctx, db.Driver(), tx.Tx, func() error {
		return fn(ctx, tx)
	})
}
