package second

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/gotomicro/egen/internal/integration"
)

type UserDAO struct {
	session interface {
		QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
		QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
		ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	}
}

type UserTxDAO struct {
	*UserDAO
}

func (dao *UserTxDAO) Rollback() error {
	tx, ok := dao.session.(*sql.Tx)
	if !ok {
		return errors.New("非事务")
	}
	return tx.Rollback()
}

func (dao *UserTxDAO) Commit() error {
	tx, ok := dao.session.(*sql.Tx)
	if !ok {
		return errors.New("非事务")
	}
	return tx.Commit()
}

func (dao *UserDAO) Begin(ctx context.Context, opts *sql.TxOptions) (*UserTxDAO, error) {
	db, ok := dao.session.(*sql.DB)
	if !ok {
		return nil, errors.New("不能在事务中开启事务")
	}
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &UserTxDAO{
		UserDAO: &UserDAO{tx},
	}, nil
}

func NewUserDAO(db *sql.DB) (*UserDAO, error) {
	return &UserDAO{db}, nil
}

func (dao *UserDAO) Insert(ctx context.Context, vals ...*integration.User) (int64, error) {
	if len(vals) == 0 || vals == nil {
		return 0, nil
	}
	var args = make([]interface{}, 0, len(vals)*(4))
	var str = ""
	for k, v := range vals {
		if k != 0 {
			str += ", "
		}
		str += "(?,?,?,?)"
		args = append(args, v.ID, v.Username, v.Password, v.Login)
	}
	sqlSen := "INSERT INTO `user_second`(`id`,`username`,`password`,`login`) VALUES" + str
	res, err := dao.session.ExecContext(ctx, sqlSen, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (dao *UserDAO) NewOne(row *sql.Row) (*integration.User, error) {
	if err := row.Err(); err != nil {
		return nil, err
	}
	var val integration.User
	err := row.Scan(&val.ID, &val.Username, &val.Password, &val.Login)
	return &val, err
}

func (dao *UserDAO) SelectByRaw(ctx context.Context, query string, args ...any) (*integration.User, error) {
	row := dao.session.QueryRowContext(ctx, query, args...)
	return dao.NewOne(row)
}

func (dao *UserDAO) SelectByWhere(ctx context.Context, where string, args ...any) (*integration.User, error) {
	s := "SELECT `id`,`username`,`password`,`login` FROM `user_second` WHERE " + where
	return dao.SelectByRaw(ctx, s, args...)
}

func (dao *UserDAO) NewBatch(rows *sql.Rows) ([]*integration.User, error) {
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var vals = make([]*integration.User, 0, 4)
	for rows.Next() {
		var val integration.User
		if err := rows.Scan(&val.ID, &val.Username, &val.Password, &val.Login); err != nil {
			return nil, err
		}
		vals = append(vals, &val)
	}
	return vals, nil
}

func (dao *UserDAO) SelectBatchByRaw(ctx context.Context, query string, args ...any) ([]*integration.User, error) {
	rows, err := dao.session.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return dao.NewBatch(rows)
}

func (dao *UserDAO) SelectBatchByWhere(ctx context.Context, where string, args ...any) ([]*integration.User, error) {
	s := "SELECT `id`,`username`,`password`,`login` FROM `user_second` WHERE " + where
	return dao.SelectBatchByRaw(ctx, s, args...)
}

func (dao *UserDAO) UpdateSpecificColsByWhere(ctx context.Context, val *integration.User, cols []string, where string, args ...any) (int64, error) {
	newArgs, colAfter := dao.quotedSpecificCol(val, cols...)
	newArgs = append(newArgs, args...)
	s := "UPDATE `user_second` SET " + colAfter + " WHERE " + where
	return dao.UpdateColsByRaw(ctx, s, newArgs...)
}

func (dao *UserDAO) UpdateNoneZeroColByWhere(ctx context.Context, val *integration.User, where string, args ...any) (int64, error) {
	newArgs, colAfter := dao.quotedNoneZero(val)
	newArgs = append(newArgs, args...)
	s := "UPDATE `user_second` SET " + colAfter + " WHERE " + where
	return dao.UpdateColsByRaw(ctx, s, newArgs...)
}

func (dao *UserDAO) UpdateNonePKColByWhere(ctx context.Context, val *integration.User, where string, args ...any) (int64, error) {
	newArgs, colAfter := dao.quotedNonePK(val)
	newArgs = append(newArgs, args...)
	s := "UPDATE `user_second` SET " + colAfter + " WHERE " + where
	return dao.UpdateColsByRaw(ctx, s, newArgs...)
}

func (dao *UserDAO) quotedNoneZero(val *integration.User) ([]interface{}, string) {
	var cols = make([]string, 0, 4)
	var args = make([]interface{}, 0, 4)
	if val.ID != 0 {
		args = append(args, val.ID)
		cols = append(cols, "`id`")
	}
	if val.Username != "" {
		args = append(args, val.Username)
		cols = append(cols, "`username`")
	}
	if val.Password != "" {
		args = append(args, val.Password)
		cols = append(cols, "`password`")
	}
	if val.Login != "" {
		args = append(args, val.Login)
		cols = append(cols, "`login`")
	}
	if len(cols) == 1 {
		cols[0] = cols[0] + "=?"
	} else {
		cols[len(cols)-1] = cols[len(cols)-1] + "=?"
	}
	return args, strings.Join(cols, "=?,")
}

func (dao *UserDAO) quotedNonePK(val *integration.User) ([]interface{}, string) {
	var cols = []string{"`username`", "`password`", "`login`"}
	var args = []interface{}{val.Username, val.Password, val.Login}
	if len(cols) == 1 {
		cols[0] = cols[0] + "=?"
	} else {
		cols[len(cols)-1] = cols[len(cols)-1] + "=?"
	}
	return args, strings.Join(cols, "=?,")
}

func (dao *UserDAO) quotedSpecificCol(val *integration.User, cols ...string) ([]interface{}, string) {
	var relation = make(map[string]interface{}, 4)
	var args = make([]interface{}, 0, 4)
	relation["id"] = val.ID
	relation["login"] = val.Login
	relation["password"] = val.Password
	relation["username"] = val.Username
	for i := 0; i < len(cols); i++ {
		args = append(args, relation[cols[i]])
		cols[i] = "`" + cols[i] + "`"
	}
	if len(cols) == 1 {
		cols[0] = cols[0] + "=?"
	} else {
		cols[len(cols)-1] = cols[len(cols)-1] + "=?"
	}
	return args, strings.Join(cols, "=?,")
}

func (dao *UserDAO) UpdateColsByRaw(ctx context.Context, query string, args ...any) (int64, error) {
	res, err := dao.session.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (dao *UserDAO) DeleteByWhere(ctx context.Context, where string, args ...any) (int64, error) {
	s := "DELETE FROM `user_second` WHERE " + where
	return dao.DeleteByRaw(ctx, s, args...)
}

func (dao *UserDAO) DeleteByRaw(ctx context.Context, query string, args ...any) (int64, error) {
	res, err := dao.session.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
