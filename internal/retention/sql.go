package retention

import (
	"fmt"

	"github.com/lib/pq"
)

func BuildCountSQL(policy Policy) (string, error) {
	if _, err := ValidatePolicy(policy); err != nil {
		return "", err
	}

	table := pq.QuoteIdentifier(policy.TableName)
	where, _ := retentionWhereSQL(policy, 1)

	return fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s`, table, where), nil
}

func BuildChunkDeleteSQL(policy Policy) (string, error) {
	spec, err := ValidatePolicy(policy)
	if err != nil {
		return "", err
	}

	table := pq.QuoteIdentifier(policy.TableName)
	idColumn := pq.QuoteIdentifier(spec.IDColumn)
	timeColumn := pq.QuoteIdentifier(policy.TimeColumn)
	where, limitArg := retentionWhereSQL(policy, 1)

	return fmt.Sprintf(`
		DELETE FROM %s
		WHERE %s IN (
		    SELECT %s
		    FROM %s
		    WHERE %s
		    ORDER BY %s ASC
		    LIMIT $%d
		)
	`, table, idColumn, idColumn, table, where, timeColumn, limitArg), nil
}

func retentionWhereSQL(policy Policy, firstArg int) (string, int) {
	timeColumn := pq.QuoteIdentifier(policy.TimeColumn)
	if policy.IntervalColumn == "" {
		return fmt.Sprintf("%s < $%d", timeColumn, firstArg), firstArg + 1
	}

	intervalColumn := pq.QuoteIdentifier(policy.IntervalColumn)
	return fmt.Sprintf("%s < $%d AND %s = $%d", timeColumn, firstArg, intervalColumn, firstArg+1), firstArg + 2
}
