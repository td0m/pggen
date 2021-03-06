package pginfer

import (
	"errors"
	"github.com/jschaf/pggen/internal/ast"
	"github.com/jschaf/pggen/internal/pg"
	"github.com/jschaf/pggen/internal/pgtest"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestInferrer_InferTypes(t *testing.T) {
	conn, cleanupFunc := pgtest.NewPostgresSchema(t, []string{
		"../../example/author/schema.sql",
	})
	defer cleanupFunc()

	tests := []struct {
		query *ast.SourceQuery
		want  TypedQuery
	}{
		{
			&ast.SourceQuery{
				Name:        "LiteralQuery",
				PreparedSQL: "SELECT 1 as one, 'foo' as two",
				ResultKind:  ast.ResultKindOne,
			},
			TypedQuery{
				Name:        "LiteralQuery",
				ResultKind:  ast.ResultKindOne,
				PreparedSQL: "SELECT 1 as one, 'foo' as two",
				Outputs: []OutputColumn{
					{PgName: "one", PgType: pg.Int4, Nullable: false},
					{PgName: "two", PgType: pg.Text, Nullable: false},
				},
			},
		},
		{
			&ast.SourceQuery{
				Name:        "FindByFirstName",
				PreparedSQL: "SELECT first_name FROM author WHERE first_name = $1;",
				ParamNames:  []string{"FirstName"},
				ResultKind:  ast.ResultKindMany,
				Doc:         newCommentGroup("--   Hello  ", "-- name: Foo"),
			},
			TypedQuery{
				Name:        "FindByFirstName",
				ResultKind:  ast.ResultKindMany,
				Doc:         []string{"Hello"},
				PreparedSQL: "SELECT first_name FROM author WHERE first_name = $1;",
				Inputs: []InputParam{
					{PgName: "FirstName", PgType: pg.Text},
				},
				Outputs: []OutputColumn{
					{PgName: "first_name", PgType: pg.Text, Nullable: false},
				},
			},
		},
		{
			&ast.SourceQuery{
				Name:        "FindByFirstNameJoin",
				PreparedSQL: "SELECT a1.first_name FROM author a1 JOIN author a2 USING (author_id) WHERE a1.first_name = $1;",
				ParamNames:  []string{"FirstName"},
				ResultKind:  ast.ResultKindMany,
				Doc:         newCommentGroup("--   Hello  ", "-- name: Foo"),
			},
			TypedQuery{
				Name:        "FindByFirstNameJoin",
				ResultKind:  ast.ResultKindMany,
				Doc:         []string{"Hello"},
				PreparedSQL: "SELECT a1.first_name FROM author a1 JOIN author a2 USING (author_id) WHERE a1.first_name = $1;",
				Inputs: []InputParam{
					{PgName: "FirstName", PgType: pg.Text},
				},
				Outputs: []OutputColumn{
					{PgName: "first_name", PgType: pg.Text, Nullable: true},
				},
			},
		},
		{
			&ast.SourceQuery{
				Name:        "DeleteAuthorByID",
				PreparedSQL: "DELETE FROM author WHERE author_id = $1;",
				ParamNames:  []string{"AuthorID"},
				ResultKind:  ast.ResultKindExec,
				Doc:         newCommentGroup("-- One", "--- - two", "-- name: Foo"),
			},
			TypedQuery{
				Name:        "DeleteAuthorByID",
				ResultKind:  ast.ResultKindExec,
				Doc:         []string{"One", "- two"},
				PreparedSQL: "DELETE FROM author WHERE author_id = $1;",
				Inputs: []InputParam{
					{PgName: "AuthorID", PgType: pg.Int4},
				},
				Outputs: nil,
			},
		},
		{
			&ast.SourceQuery{
				Name:        "DeleteAuthorByIDReturning",
				PreparedSQL: "DELETE FROM author WHERE author_id = $1 RETURNING author_id, first_name;",
				ParamNames:  []string{"AuthorID"},
				ResultKind:  ast.ResultKindMany,
			},
			TypedQuery{
				Name:        "DeleteAuthorByIDReturning",
				ResultKind:  ast.ResultKindMany,
				PreparedSQL: "DELETE FROM author WHERE author_id = $1 RETURNING author_id, first_name;",
				Inputs: []InputParam{
					{PgName: "AuthorID", PgType: pg.Int4},
				},
				Outputs: []OutputColumn{
					{PgName: "author_id", PgType: pg.Int4, Nullable: false},
					{PgName: "first_name", PgType: pg.Text, Nullable: false},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.query.Name, func(t *testing.T) {
			inferrer := NewInferrer(conn)
			got, err := inferrer.InferTypes(tt.query)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tt.want, got, "typed query should match")
		})
	}
}

func TestInferrer_InferTypes_Error(t *testing.T) {
	conn, cleanupFunc := pgtest.NewPostgresSchema(t, []string{
		"../../example/author/schema.sql",
	})
	defer cleanupFunc()

	tests := []struct {
		query *ast.SourceQuery
		want  error
	}{
		{
			&ast.SourceQuery{
				Name:        "DeleteAuthorByIDMany",
				PreparedSQL: "DELETE FROM author WHERE author_id = $1;",
				ParamNames:  []string{"AuthorID"},
				ResultKind:  ast.ResultKindMany,
			},
			errors.New("query DeleteAuthorByIDMany has incompatible result kind :many; " +
				"the query doesn't return any rows; " +
				"use :exec if query shouldn't return rows"),
		},
		{
			&ast.SourceQuery{
				Name:        "DeleteAuthorByIDOne",
				PreparedSQL: "DELETE FROM author WHERE author_id = $1;",
				ParamNames:  []string{"AuthorID"},
				ResultKind:  ast.ResultKindOne,
			},
			errors.New(
				"query DeleteAuthorByIDOne has incompatible result kind :one; " +
					"the query doesn't return any rows; " +
					"use :exec if query shouldn't return rows"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.query.Name, func(t *testing.T) {
			inferrer := NewInferrer(conn)
			got, err := inferrer.InferTypes(tt.query)
			assert.Equal(t, TypedQuery{}, got, "InferTypes should error and return empty TypedQuery struct")
			assert.Equal(t, tt.want, err, "InferType error should match")
		})
	}
}

func newCommentGroup(lines ...string) *ast.CommentGroup {
	cs := make([]*ast.LineComment, len(lines))
	for i, line := range lines {
		cs[i] = &ast.LineComment{Text: line}
	}
	return &ast.CommentGroup{List: cs}
}
