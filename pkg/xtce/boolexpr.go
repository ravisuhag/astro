package xtce

import (
	"bytes"
	"fmt"
)

// Evaluating a BooleanExpression.
//
// A Comparison says "this field equals this value". A ComparisonList says
// several of those, all of which must hold. A BooleanExpression is the third
// form, and the only one that can express an alternative: it is a tree of
// Conditions joined by AND and OR to any depth, and a Condition's right-hand
// side may be a second parameter rather than a constant.
//
// The structure comes from the schema's BooleanExpressionType,
// ANDedConditionsType and ORedConditionsType. AND groups hold OR groups and
// the other way round, which is how alternation and conjunction interleave
// without the format needing parentheses.

// satisfiesExpression reports whether a packet meets a boolean expression.
func (s *SpaceSystem) satisfiesExpression(
	container *SequenceContainer, expression *BooleanExpression, packet []byte,
) (bool, error) {
	switch {
	case expression.Condition != nil:
		return s.satisfiesCondition(container, expression.Condition, packet)

	case expression.ANDedConditions != nil:
		return s.satisfiesGroup(container, expression.ANDedConditions, true, packet)

	case expression.ORedConditions != nil:
		return s.satisfiesGroup(container, expression.ORedConditions, false, packet)

	default:
		// A BooleanExpression element with none of the three inside it. The
		// schema requires one, so this is a malformed database. Reading it as
		// "always true" would let the container swallow every packet.
		return false, fmt.Errorf("%w: container %q has an empty BooleanExpression",
			ErrUnsupportedCriteria, container.Name)
	}
}

// satisfiesGroup evaluates one AND or OR group.
//
// The two differ only in what makes the answer known early, so they share the
// walk. An empty group is refused rather than defaulted: the schema requires
// at least two members, and the identity of an empty AND is true, which would
// make a malformed group match everything.
func (s *SpaceSystem) satisfiesGroup(
	container *SequenceContainer, group *ConditionGroup, all bool, packet []byte,
) (bool, error) {
	members := len(group.Conditions) + len(group.ANDedConditions) + len(group.ORedConditions)
	if members == 0 {
		return false, fmt.Errorf("%w: container %q has an empty condition group",
			ErrUnsupportedCriteria, container.Name)
	}

	// decide folds one member in and says whether the answer is settled.
	// An AND is settled by a false, an OR by a true.
	settled := func(ok bool) bool { return ok != all }

	for i := range group.Conditions {
		ok, err := s.satisfiesCondition(container, &group.Conditions[i], packet)
		if err != nil {
			return false, err
		}
		if settled(ok) {
			return ok, nil
		}
	}
	for i := range group.ANDedConditions {
		ok, err := s.satisfiesGroup(container, &group.ANDedConditions[i], true, packet)
		if err != nil {
			return false, err
		}
		if settled(ok) {
			return ok, nil
		}
	}
	for i := range group.ORedConditions {
		ok, err := s.satisfiesGroup(container, &group.ORedConditions[i], false, packet)
		if err != nil {
			return false, err
		}
		if settled(ok) {
			return ok, nil
		}
	}

	// Nothing settled it, so every member agreed with the group's join: all
	// true for an AND, all false for an OR.
	return all, nil
}

// satisfiesCondition evaluates one Condition against a packet.
func (s *SpaceSystem) satisfiesCondition(
	container *SequenceContainer, condition *Condition, packet []byte,
) (bool, error) {
	left, ok := condition.Left()
	if !ok {
		return false, fmt.Errorf("%w: container %q has a Condition with no parameter",
			ErrUnsupportedCriteria, container.Name)
	}
	if left.Instance != 0 {
		// The value in a different packet. Deciding it needs the stream, not
		// this packet.
		return false, fmt.Errorf("%w: a Condition on %q with instance %d",
			ErrUnsupportedCriteria, left.ParameterRef, left.Instance)
	}
	if condition.ComparisonOperator == "" {
		return false, fmt.Errorf("%w: a Condition on %q with no ComparisonOperator",
			ErrUnsupportedCriteria, left.ParameterRef)
	}

	actual, field, present, err := s.readCriterionParameter(
		container, left.ParameterRef, left.Calibrated(), packet)
	if err != nil || !present {
		return false, err
	}

	// The right-hand side is either a second parameter or a literal, and the
	// schema makes those a choice.
	if right, isParameter := condition.Right(); isParameter {
		if right.Instance != 0 {
			return false, fmt.Errorf("%w: a Condition against %q with instance %d",
				ErrUnsupportedCriteria, right.ParameterRef, right.Instance)
		}

		other, _, otherPresent, err := s.readCriterionParameter(
			container, right.ParameterRef, right.Calibrated(), packet)
		if err != nil || !otherPresent {
			return false, err
		}
		return compareValues(actual, other, condition.ComparisonOperator)
	}

	if condition.Value == nil {
		return false, fmt.Errorf("%w: a Condition on %q with neither a second parameter nor a Value",
			ErrUnsupportedCriteria, left.ParameterRef)
	}
	return evaluate(actual, condition.ComparisonOperator, *condition.Value, field)
}

// compareValues applies an operator to two values read from the same packet.
//
// This is the case a Comparison cannot express, so it does not go through
// evaluate: there is no text to parse, just two decoded values that have to
// be of comparable kinds. Comparing a number against a label would be a
// database error rather than a false result, so it is reported.
func compareValues(left, right any, operator string) (bool, error) {
	switch first := left.(type) {
	case string:
		second, ok := right.(string)
		if !ok {
			return false, fmt.Errorf("%w: comparing the text %q against a %T",
				ErrUnsupportedCriteria, first, right)
		}
		// A label has no order, so only equality means anything.
		switch operator {
		case "==":
			return first == second, nil
		case "!=":
			return first != second, nil
		default:
			return false, fmt.Errorf("%w: operator %q on two text values",
				ErrUnsupportedCriteria, operator)
		}

	case []byte:
		second, ok := right.([]byte)
		if !ok {
			return false, fmt.Errorf("%w: comparing a binary value against a %T",
				ErrUnsupportedCriteria, right)
		}
		switch operator {
		case "==":
			return bytes.Equal(first, second), nil
		case "!=":
			return !bytes.Equal(first, second), nil
		default:
			return false, fmt.Errorf("%w: operator %q on two binary values",
				ErrUnsupportedCriteria, operator)
		}

	default:
		a, ok := toFloat(left)
		if !ok {
			return false, fmt.Errorf("%w: comparing a %T", ErrUnsupportedCriteria, left)
		}
		b, ok := toFloat(right)
		if !ok {
			return false, fmt.Errorf("%w: comparing against a %T", ErrUnsupportedCriteria, right)
		}
		return compareFloats(a, b, operator)
	}
}

// compareFloats applies one of the schema's six operators to two numbers.
func compareFloats(a, b float64, operator string) (bool, error) {
	switch operator {
	case "==":
		return a == b, nil
	case "!=":
		return a != b, nil
	case "<":
		return a < b, nil
	case "<=":
		return a <= b, nil
	case ">":
		return a > b, nil
	case ">=":
		return a >= b, nil
	default:
		return false, fmt.Errorf("%w: comparison operator %q", ErrUnsupportedCriteria, operator)
	}
}
