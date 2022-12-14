package evaluator

import (
	"bananascript/src/parser"
	"bananascript/src/token"
	"fmt"
	"reflect"
)

func Eval(node parser.Node, environment *Environment) Object {
	switch node := node.(type) {
	case *parser.Program:
		return evalProgram(node, environment)
	case *parser.ExpressionStatement:
		return Eval(node.Expression, environment)
	case *parser.StringLiteral:
		return &StringObject{Value: node.Value}
	case *parser.IntegerLiteral:
		return &IntegerObject{Value: node.Value}
	case *parser.FloatLiteral:
		return &FloatObject{Value: node.Value}
	case *parser.BooleanLiteral:
		return &BooleanObject{Value: node.Value}
	case *parser.NullLiteral:
		return &NullObject{}
	case *parser.VoidLiteral:
		return nil
	case *parser.Identifier:
		return evalIdentifierExpression(node, environment)
	case *parser.InfixExpression:
		return evalInfixExpression(node, environment)
	case *parser.PrefixExpression:
		return evalPrefixExpression(node, environment)
	case *parser.CallExpression:
		return evalCallExpression(node, environment)
	case *parser.AssignmentExpression:
		return evalAssignmentExpression(node, environment)
	case *parser.LetStatement:
		return evalLetStatement(node, environment)
	case *parser.FunctionDefinitionStatement:
		return evalFunctionDefinitionStatement(node, environment)
	case *parser.ReturnStatement:
		return evalReturnStatement(node, environment)
	case *parser.BlockStatement:
		return evalBlockStatement(node, environment)
	case *parser.IfStatement:
		return evalIfStatement(node, environment)
	case *parser.WhileStatement:
		return evalWhileStatement(node, environment)
	case *parser.IncrementExpression:
		return evalIncrementExpression(node, environment)
	case *parser.MemberAccessExpression:
		return evalMemberAccessExpression(node, environment)
	case *parser.TypeDefinitionStatement:
		return nil
	}
	return NewError("Unknown node (%T)", node)
}

func evalProgram(program *parser.Program, environment *Environment) Object {
	newEnvironment := ExtendEnvironment(environment, program.Context)
	for _, statement := range program.Statements {
		result := Eval(statement, newEnvironment)
		switch result := result.(type) {
		case *ErrorObject:
			return result
		}
	}
	return nil
}

func evalPrefixExpression(prefixExpression *parser.PrefixExpression, environment *Environment) Object {

	object := Eval(prefixExpression.Expression, environment)
	if isError(object) {
		return object
	}

	switch prefixExpression.Operator {
	case token.Bang:
		return &BooleanObject{Value: !implicitBoolConversion(object)}
	case token.Minus:
		switch object := object.(type) {
		case *IntegerObject:
			return &IntegerObject{Value: -object.Value}
		case *FloatObject:
			return &FloatObject{Value: -object.Value}
		}
	}

	return NewError("Unknown prefix operator")
}

func evalInfixExpression(infixExpression *parser.InfixExpression, environment *Environment) Object {

	leftObject := Eval(infixExpression.Left, environment)
	if isError(leftObject) {
		return leftObject
	}
	if infixExpression.Operator == token.LogicalAnd && !implicitBoolConversion(leftObject) {
		return &BooleanObject{Value: false}
	} else if infixExpression.Operator == token.LogicalOr && implicitBoolConversion(leftObject) {
		return &BooleanObject{Value: true}
	}

	rightObject := Eval(infixExpression.Right, environment)
	if isError(rightObject) {
		return rightObject
	}
	if infixExpression.Operator == token.LogicalAnd || infixExpression.Operator == token.LogicalOr {
		return &BooleanObject{Value: implicitBoolConversion(rightObject)}
	}

	switch infixExpression.Operator {
	case token.EQ:
		return &BooleanObject{Value: evalEquals(leftObject, rightObject)}
	case token.NEQ:
		return &BooleanObject{Value: !evalEquals(leftObject, rightObject)}
	case token.LT:
		return evalNumericInfix(
			leftObject, rightObject,
			func(left int64, right int64) Object { return &BooleanObject{Value: left < right} },
			func(left float64, right float64) Object { return &BooleanObject{Value: left < right} },
		)
	case token.GT:
		return evalNumericInfix(
			leftObject, rightObject,
			func(left int64, right int64) Object { return &BooleanObject{Value: left > right} },
			func(left float64, right float64) Object { return &BooleanObject{Value: left > right} },
		)
	case token.LTE:
		return evalNumericInfix(
			leftObject, rightObject,
			func(left int64, right int64) Object { return &BooleanObject{Value: left <= right} },
			func(left float64, right float64) Object { return &BooleanObject{Value: left <= right} },
		)
	case token.GTE:
		return evalNumericInfix(
			leftObject, rightObject,
			func(left int64, right int64) Object { return &BooleanObject{Value: left >= right} },
			func(left float64, right float64) Object { return &BooleanObject{Value: left >= right} },
		)
	case token.Plus:
		_, leftIsString := leftObject.(*StringObject)
		_, rightIsString := rightObject.(*StringObject)
		if leftIsString || rightIsString {
			return &StringObject{Value: leftObject.ToString() + rightObject.ToString()}
		}
		return evalNumericInfix(
			leftObject, rightObject,
			func(left int64, right int64) Object { return &IntegerObject{Value: left + right} },
			func(left float64, right float64) Object { return &FloatObject{Value: left + right} },
		)
	case token.Minus:
		return evalNumericInfix(
			leftObject, rightObject,
			func(left int64, right int64) Object { return &IntegerObject{Value: left - right} },
			func(left float64, right float64) Object { return &FloatObject{Value: left - right} },
		)
	case token.Slash:
		return evalNumericInfix(
			leftObject, rightObject,
			func(left int64, right int64) Object { return &IntegerObject{Value: left / right} },
			func(left float64, right float64) Object { return &FloatObject{Value: left / right} },
		)
	case token.Star:
		return evalNumericInfix(
			leftObject, rightObject,
			func(left int64, right int64) Object { return &IntegerObject{Value: left * right} },
			func(left float64, right float64) Object { return &FloatObject{Value: left * right} },
		)
	default:
		return NewError("Unknown infix operator")
	}
}

func evalEquals(left Object, right Object) bool {
	return reflect.DeepEqual(left, right)
}

func evalNumericInfix(left Object, right Object, intConstructor func(left int64, right int64) Object, floatConstructor func(left float64, right float64) Object) Object {
	switch left := left.(type) {
	case *IntegerObject:
		switch right := right.(type) {
		case *IntegerObject:
			return intConstructor(left.Value, right.Value)
		case *FloatObject:
			return floatConstructor(float64(left.Value), right.Value)
		}
	case *FloatObject:
		switch right := right.(type) {
		case *IntegerObject:
			return floatConstructor(left.Value, float64(right.Value))
		case *FloatObject:
			return floatConstructor(left.Value, right.Value)
		}
	}
	return NewError("Invalid infix operator")
}

func evalAssignmentExpression(assignmentExpression *parser.AssignmentExpression, environment *Environment) Object {

	object := Eval(assignmentExpression.Expression, environment)
	if isError(object) {
		return object
	}

	name := assignmentExpression.Name.Value
	if object, ok := environment.AssignObject(name, object); ok {
		return object
	} else {
		return NewError("Cannot resolve variable")
	}
}

func evalCallExpression(callExpression *parser.CallExpression, environment *Environment) Object {
	function := Eval(callExpression.Function, environment)
	switch function := function.(type) {
	case *ErrorObject:
		return function
	case Function:
		argumentObjects := make([]Object, 0)
		for _, argument := range callExpression.Arguments {
			argumentObjects = append(argumentObjects, Eval(argument, environment))
		}
		returned := function.Execute(argumentObjects)
		switch returned := returned.(type) {
		case *ReturnObject:
			return returned.Object
		default:
			return returned
		}
	default:
		return NewError("Cannot call non-function")
	}
}

func evalIdentifierExpression(identifier *parser.Identifier, environment *Environment) Object {
	if object, exists := environment.GetObject(identifier.Value); exists {
		return object
	} else {
		return NewError("Cannot resolve identifier")
	}
}

func evalLetStatement(letStatement *parser.LetStatement, environment *Environment) Object {

	object := Eval(letStatement.Value, environment)
	if isError(object) {
		return object
	}

	name := letStatement.Name.Value
	environment.DefineObject(name, object)
	return nil
}

func evalFunctionDefinitionStatement(funcStatement *parser.FunctionDefinitionStatement, environment *Environment) Object {

	name := funcStatement.Name.Value

	identifiers := make([]*parser.Identifier, 0)
	for _, parameter := range funcStatement.Parameters {
		identifiers = append(identifiers, parameter.Name)
	}

	object := &FunctionObject{
		Parameters:   identifiers,
		Body:         funcStatement.Body,
		Environment:  environment,
		Context:      funcStatement.FunctionContext,
		FunctionType: funcStatement.FunctionType,
	}

	if funcStatement.ThisType != nil {
		environment.DefineTypeMember(funcStatement.ThisType, name, object)
	} else {
		environment.DefineObject(name, object)
	}
	return nil
}

func evalReturnStatement(returnStatement *parser.ReturnStatement, environment *Environment) Object {
	object := Eval(returnStatement.Expression, environment)
	if isError(object) {
		return object
	}
	return &ReturnObject{Object: object}
}

func evalBlockStatement(blockStatement *parser.BlockStatement, environment *Environment) Object {
	newEnvironment := ExtendEnvironment(environment, blockStatement.Context)

	for _, statement := range blockStatement.Statements {
		object := Eval(statement, newEnvironment)
		if object != nil {
			switch object := object.(type) {
			case *ErrorObject, *ReturnObject:
				return object
			default:
				continue
			}
		}
	}

	return nil
}

func evalIfStatement(ifStatement *parser.IfStatement, environment *Environment) Object {
	condition := Eval(ifStatement.Condition, environment)
	if isError(condition) {
		return condition
	}
	var object Object
	if implicitBoolConversion(condition) {
		object = Eval(ifStatement.Statement, ExtendEnvironment(environment, ifStatement.StatementContext))
	} else if ifStatement.Alternative != nil {
		object = Eval(ifStatement.Alternative, ExtendEnvironment(environment, ifStatement.AlternativeContext))
	}
	switch object.(type) {
	case *ErrorObject, *ReturnObject:
		return object
	default:
		return nil
	}
}

func evalWhileStatement(whileStatement *parser.WhileStatement, environment *Environment) Object {
	for {
		condition := Eval(whileStatement.Condition, environment)
		if isError(condition) {
			return condition
		}
		if !implicitBoolConversion(condition) {
			return nil
		}
		object := Eval(whileStatement.Statement, ExtendEnvironment(environment, whileStatement.StatementContext))
		switch object := object.(type) {
		case *ErrorObject, *ReturnObject:
			return object
		default:
			continue
		}
	}
}

func evalIncrementExpression(incrementExpression *parser.IncrementExpression, environment *Environment) Object {

	object, exists := environment.GetObject(incrementExpression.Name.Value)
	if !exists {
		return NewError("Cannot resolve identifier")
	}

	switch object := object.(type) {
	case *IntegerObject:
		oldValue := object.Value
		if incrementExpression.Operator == token.Increment {
			object.Value++
		} else {
			object.Value--
		}
		if incrementExpression.Pre {
			return object
		} else {
			return &IntegerObject{Value: oldValue}
		}
	case *FloatObject:
		oldValue := object.Value
		if incrementExpression.Operator == token.Increment {
			object.Value++
		} else {
			object.Value--
		}
		if incrementExpression.Pre {
			return object
		} else {
			return &FloatObject{Value: oldValue}
		}
	}

	return NewError("Cannot increment non-int")
}

func evalMemberAccessExpression(memberAccessExpression *parser.MemberAccessExpression, environment *Environment) Object {

	object := Eval(memberAccessExpression.Expression, environment)
	if isError(object) {
		return object
	}

	member, ok := environment.GetTypeMember(object, object.Type(), memberAccessExpression.Member.Value)
	if !ok {
		return NewError("Member %s does not exist", memberAccessExpression.Member.Value)
	}

	switch member := member.(type) {
	case Function:
		return member.With(object)
	default:
		return member
	}
}

func implicitBoolConversion(object Object) bool {
	switch object := object.(type) {
	case *BooleanObject:
		return object.Value
	case *IntegerObject:
		return object.Value != 0
	case *FloatObject:
		return object.Value != 0
	case *StringObject:
		return len(object.Value) != 0
	default:
		return true
	}
}

func NewError(format string, args ...interface{}) *ErrorObject {
	return &ErrorObject{Message: fmt.Sprintf(format, args...)}
}

func isError(object Object) bool {
	_, isError := object.(*ErrorObject)
	return isError
}
