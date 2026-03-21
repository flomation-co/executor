package sql_common

import (
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	. "github.com/onsi/gomega"
)

func TestExecuteQuerySelect(t *testing.T) {
	RegisterTestingT(t)

	db, mock, err := sqlmock.New()
	Expect(err).ToNot(HaveOccurred())
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "name", "email"}).
		AddRow(1, "Alice", "alice@example.com").
		AddRow(2, "Bob", "bob@example.com")
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	result, err := ExecuteQuery(db, "SELECT id, name, email FROM users")
	Expect(err).ToNot(HaveOccurred())
	Expect(result).ToNot(BeNil())

	results := result["results"].([]map[string]interface{})
	Expect(results).To(HaveLen(2))
	Expect(results[0]["name"]).To(Equal("Alice"))
	Expect(results[1]["email"]).To(Equal("bob@example.com"))
	Expect(result["row_count"]).To(Equal(2))
}

func TestExecuteQueryDML(t *testing.T) {
	RegisterTestingT(t)

	db, mock, err := sqlmock.New()
	Expect(err).ToNot(HaveOccurred())
	defer db.Close()

	mock.ExpectExec("INSERT INTO").WillReturnResult(sqlmock.NewResult(1, 3))

	result, err := ExecuteQuery(db, "INSERT INTO users (name) VALUES ('Alice'), ('Bob'), ('Charlie')")
	Expect(err).ToNot(HaveOccurred())
	Expect(result).ToNot(BeNil())
	Expect(result["results"]).To(BeNil())
	Expect(result["row_count"]).To(Equal(int64(3)))
}

func TestExecuteQueryEmptyResultSet(t *testing.T) {
	RegisterTestingT(t)

	db, mock, err := sqlmock.New()
	Expect(err).ToNot(HaveOccurred())
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "name"})
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	result, err := ExecuteQuery(db, "SELECT id, name FROM users WHERE 1=0")
	Expect(err).ToNot(HaveOccurred())
	Expect(result).ToNot(BeNil())

	results := result["results"].([]map[string]interface{})
	Expect(results).ToNot(BeNil())
	Expect(results).To(HaveLen(0))
	Expect(result["row_count"]).To(Equal(0))
}

func TestExecuteQueryError(t *testing.T) {
	RegisterTestingT(t)

	db, mock, err := sqlmock.New()
	Expect(err).ToNot(HaveOccurred())
	defer db.Close()

	mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("syntax error"))

	result, err := ExecuteQuery(db, "SELECT * FROM nonexistent")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("syntax error"))
	Expect(result).To(BeNil())
}

func TestExecuteQueryWithCTE(t *testing.T) {
	RegisterTestingT(t)

	db, mock, err := sqlmock.New()
	Expect(err).ToNot(HaveOccurred())
	defer db.Close()

	rows := sqlmock.NewRows([]string{"total"}).AddRow(42)
	mock.ExpectQuery("WITH").WillReturnRows(rows)

	result, err := ExecuteQuery(db, "WITH cte AS (SELECT count(*) as total FROM users) SELECT total FROM cte")
	Expect(err).ToNot(HaveOccurred())
	Expect(result).ToNot(BeNil())

	results := result["results"].([]map[string]interface{})
	Expect(results).To(HaveLen(1))
	Expect(results[0]["total"]).To(Equal(int64(42)))
}

func TestExecuteQueryByteConversion(t *testing.T) {
	RegisterTestingT(t)

	db, mock, err := sqlmock.New()
	Expect(err).ToNot(HaveOccurred())
	defer db.Close()

	rows := sqlmock.NewRows([]string{"data"}).AddRow([]byte("binary content"))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	result, err := ExecuteQuery(db, "SELECT data FROM files")
	Expect(err).ToNot(HaveOccurred())

	results := result["results"].([]map[string]interface{})
	Expect(results[0]["data"]).To(Equal("binary content"))
}
