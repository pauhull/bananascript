package parser

import (
	"bananascript/src/token"
	"reflect"
)

func (parser *Parser) parseStatement(context *Context) Statement {
	switch parser.current().Type {
	case token.Let:
		return parser.parseLetStatement(context)
	case token.Return:
		return parser.parseReturnStatement(context)
	case token.Func:
		return parser.parseFunctionDefinitionStatement(context)
	case token.LBrace:
		return parser.parseBlockStatement(context)
	case token.If:
		return parser.parseIfStatement(context)
	case token.While:
		return parser.parseWhileStatement(context)
	default:
		return parser.parseExpressionStatement(context)
	}
}

func (parser *Parser) parseExpressionStatement(context *Context) *ExpressionStatement {
	statement := &ExpressionStatement{}
	statement.Expression = parser.parseExpression(context, Lowest)

	if !parser.assertNext(token.Semi) {
		return nil
	}

	return statement
}

func (parser *Parser) parseReturnStatement(context *Context) *ReturnStatement {
	statement := &ReturnStatement{ReturnToken: parser.consume()}

	if parser.current().Type == token.Semi {
		statement.Expression = &VoidLiteral{}
		return statement
	}

	statement.Expression = parser.parseExpression(context, Lowest)
	if !parser.assertNext(token.Semi) {
		return nil
	}
	return statement
}

func (parser *Parser) parseBlockStatement(context *Context) *BlockStatement {

	openingBrace := parser.consume()
	statements := make([]Statement, 0)

	for parser.current().Type != token.EOF && parser.current().Type != token.RBrace {
		if parser.current().Type == token.Semi {
			parser.consume()
			continue
		}

		statement := parser.parseStatement(context)
		if statement != nil && !reflect.ValueOf(statement).IsNil() {
			statements = append(statements, statement)
		}
		parser.consume()
	}

	rBraceToken := parser.current()
	if rBraceToken.Type != token.RBrace {
		parser.error(openingBrace, "Unclosed block")
		rBraceToken = nil
	}

	return &BlockStatement{Statements: statements, LBraceToken: openingBrace, RBraceToken: rBraceToken}
}

func (parser *Parser) parseLetStatement(context *Context) *LetStatement {
	statement := &LetStatement{LetToken: parser.current()}
	if !parser.assertNext(token.Ident) {
		return nil
	}

	identToken := parser.current()
	name := identToken.Literal
	statement.Name = &Identifier{IdentToken: parser.current(), Value: name}
	assignmentToken := token.Define

	if parser.peek().Type == token.Colon {
		parser.consume()
		typeToken := parser.peek()
		statement.Type = parser.parseType()
		if statement.Type == nil {
			return nil
		}
		if never, isNever := statement.Type.(*NeverType); isNever {
			parser.error(typeToken, never.Message)
		}
		assignmentToken = token.Assign
	}

	if parser.peek().Type != token.Semi {
		if !parser.assertNext(assignmentToken) {
			return nil
		}
		parser.consume()
		statement.Value = parser.parseExpression(context, Lowest)
	} else {
		statement.Value = &NullLiteral{}
	}

	if !parser.assertNext(token.Semi) {
		return nil
	}

	if _, exists := context.GetInThisScope(name); exists {
		parser.error(identToken, "Cannot redefine '%s'", name)
		return nil
	}

	inferredType := statement.Value.Type(context)
	if statement.Type == nil {
		statement.Type = inferredType
	} else if !statement.Type.IsAssignable(inferredType) {
		erroneousToken := statement.Value.Token()
		if erroneousToken == nil {
			erroneousToken = parser.current()
		}
		if !isNever(statement.Type) {
			parser.error(erroneousToken, "Type '%s' is not assignable to '%s'", inferredType.ToString(),
				statement.Type.ToString())
		}
		return nil
	}

	context.Define(name, statement.Type)
	return statement
}

func (parser *Parser) parseFunctionDefinitionStatement(context *Context) *FunctionDefinitionStatement {

	statement := &FunctionDefinitionStatement{FuncToken: parser.current()}
	if !parser.assertNext(token.Ident) {
		return nil
	}
	name := parser.current().Literal
	statement.Name = &Identifier{IdentToken: parser.current(), Value: name}

	if !parser.assertNext(token.LParen) {
		return nil
	}

	statement.Parameters = parser.parseParameterList()
	if statement.Parameters == nil {
		return nil
	}

	if parser.peek().Type == token.LBrace {
		parser.consume()
		statement.ReturnType = &VoidType{}
	} else {
		statement.ReturnType = parser.parseType()
		if statement.ReturnType == nil || !parser.assertNext(token.LBrace) {
			return nil
		}
	}

	parameterTypes := make([]Type, 0)
	functionContext := ExtendContext(context)
	functionContext.returnType = statement.ReturnType
	for _, parameter := range statement.Parameters {
		parameterTypes = append(parameterTypes, parameter.Type)
		functionContext.Define(parameter.Name.Value, parameter.Type)
	}

	context.Define(name, &FunctionType{
		ParameterTypes: parameterTypes,
		ReturnType:     statement.ReturnType,
	})

	statement.Body = parser.parseBlockStatement(CloneContext(functionContext))
	if statement.Body == nil {
		return nil
	}

	if _, isVoid := statement.ReturnType.(*VoidType); !isVoid {
		if returns := parser.doesReturn(CloneContext(functionContext), statement.Body); !returns {
			erroneousToken := statement.Body.RBraceToken
			if erroneousToken == nil {
				erroneousToken = statement.Body.LBraceToken
			}
			parser.error(erroneousToken, "Missing return statement")
		}
	}

	return statement
}

func (parser *Parser) parseIfStatement(context *Context) *IfStatement {

	statement := &IfStatement{IfToken: parser.consume()}

	statement.Condition = parser.parseExpression(context, Lowest)
	parser.consume()

	statement.Statement = parser.parseStatement(ExtendContext(context))

	if parser.peek().Type == token.Else {
		parser.consume()
		parser.consume()
		statement.Alternative = parser.parseStatement(ExtendContext(context))
	}

	return statement
}

func (parser *Parser) parseWhileStatement(context *Context) *WhileStatement {

	statement := &WhileStatement{WhileToken: parser.consume()}

	statement.Condition = parser.parseExpression(context, Lowest)
	parser.consume()

	statement.Statement = parser.parseStatement(ExtendContext(context))

	return statement
}
