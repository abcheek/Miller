mlr --nidx put '$1 = sub($1, "ab(c)?d(..)g", "ab<<\1>>d<<\2>>g")' ./reg-test/cases-pending-go-port/c-dsl-filter-pattern-action/0076.input
