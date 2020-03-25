package main

import (
	"fmt"
)

// getVar goes through all symbol varTables recursively and looks for an entry for the given variable name v
func (s *SymbolTable) getVar(v string) (SymbolVarEntry, bool) {
	if s == nil {
		return SymbolVarEntry{}, false
	}
	if variable, ok := s.varTable[v]; ok {
		return variable, true
	}
	return s.parent.getVar(v)
}

// isLocalVar only searches the immediate local symbol varTable
func (s *SymbolTable) isLocalVar(v string) bool {
	if s == nil {
		return false
	}
	_, ok := s.varTable[v]
	return ok
}

func (s *SymbolTable) setVar(v string, t Type) {
	s.varTable[v] = SymbolVarEntry{t, ""}
}

func (s *SymbolTable) setFun(name string, argTypes, returnTypes []Type) {
	s.funTable[name] = SymbolFunEntry{argTypes, returnTypes, ""}
}

func (s *SymbolTable) isLocalFun(name string) bool {
	if s == nil {
		return false
	}
	_, ok := s.funTable[name]
	return ok
}

func (s *SymbolTable) getFun(name string) (SymbolFunEntry, bool) {
	if s == nil {
		return SymbolFunEntry{}, false
	}
	if fun, ok := s.funTable[name]; ok {
		return fun, true
	}
	return s.parent.getFun(name)
}

func (s *SymbolTable) setAsmName(v string, asmName string) {
	if s == nil {
		fmt.Println("Could not set asm variable name in symbol varTable!")
		return
	}
	if _, ok := s.varTable[v]; ok {

		tmp := s.varTable[v]
		tmp.varName = asmName
		s.varTable[v] = tmp
		return
	}
	s.parent.setAsmName(v, asmName)
}

func analyzeUnaryOp(unaryOp UnaryOp, symbolTable *SymbolTable) (Expression, error) {
	expression, err := analyzeExpression(unaryOp.expr, symbolTable)
	if err != nil {
		return unaryOp, err
	}
	unaryOp.expr = expression

	t := expression.getExpressionType()

	switch unaryOp.operator {
	case OP_NEGATIVE:
		if t != TYPE_FLOAT && t != TYPE_INT {
			return nil, fmt.Errorf("%w[%v:%v] - Unary '-' expression must be float or int, but is: %v", ErrCritical, unaryOp.line, unaryOp.column, unaryOp)
		}
		unaryOp.opType = expression.getExpressionType()
		return unaryOp, nil
	case OP_NOT:
		if t != TYPE_BOOL {
			return nil, fmt.Errorf("%w[%v:%v] - Unary '!' expression must be bool, but is: %v", ErrCritical, unaryOp.line, unaryOp.column, unaryOp)
		}
		unaryOp.opType = TYPE_BOOL
		return unaryOp, nil
	}
	return nil, fmt.Errorf("%w[%v:%v] - Unknown unary expression: %v", ErrCritical, unaryOp.line, unaryOp.column, unaryOp)
}

func analyzeBinaryOp(binaryOp BinaryOp, symbolTable *SymbolTable) (Expression, error) {

	// Re-order expression, if the expression is not fixed and the priority is of the operator is not according to the priority
	// The priority of an operator must be equal or higher in (right) sub-trees (as they are evaluated first).
	if tmpE, ok := binaryOp.rightExpr.(BinaryOp); ok {
		if binaryOp.operator.priority() < tmpE.operator.priority() && !tmpE.fixed {
			newChild := binaryOp
			newChild.rightExpr = tmpE.leftExpr
			tmpE.leftExpr = newChild
			binaryOp = tmpE
		}
	}

	leftExpression, err := analyzeExpression(binaryOp.leftExpr, symbolTable)
	if err != nil {
		return binaryOp, err
	}
	binaryOp.leftExpr = leftExpression

	rightExpression, err := analyzeExpression(binaryOp.rightExpr, symbolTable)
	if err != nil {
		return binaryOp, err
	}
	binaryOp.rightExpr = rightExpression

	tLeft := binaryOp.leftExpr.getExpressionType()
	tRight := binaryOp.rightExpr.getExpressionType()

	// Check types only after we possibly rearranged the expression!
	if binaryOp.leftExpr.getExpressionType() != binaryOp.rightExpr.getExpressionType() {
		return binaryOp, fmt.Errorf(
			"%w[%v:%v] - BinaryOp '%v' expected same type, got: '%v', '%v'",
			ErrCritical, binaryOp.line, binaryOp.column, binaryOp.operator, tLeft, tRight,
		)
	}

	// We match all types explicitely to make sure that this still works or creates an error when we introduce new types
	// that are not considered yet!
	switch binaryOp.operator {
	case OP_AND, OP_OR:
		binaryOp.opType = TYPE_BOOL
		// We know left and right are the same type, so only compare left here.
		if tLeft != TYPE_BOOL {
			return binaryOp, fmt.Errorf(
				"%w[%v:%v] - BinaryOp '%v' needs bool, got: '%v'",
				ErrCritical, binaryOp.line, binaryOp.column, binaryOp.operator, tLeft,
			)
		}
		//return binaryOp, TYPE_BOOL, nil
	case OP_PLUS, OP_MINUS, OP_MULT, OP_DIV:

		if tLeft == TYPE_FLOAT {
			binaryOp.opType = TYPE_FLOAT
		} else {
			binaryOp.opType = TYPE_INT
		}
		if tLeft != TYPE_FLOAT && tLeft != TYPE_INT {
			return binaryOp, fmt.Errorf(
				"%w[%v:%v] - BinaryOp '%v' needs int/float, got: '%v'",
				ErrCritical, binaryOp.line, binaryOp.column, binaryOp.operator, tLeft,
			)
		}
		//return binaryOp, tLeft, nil
	case OP_LE, OP_GE, OP_LESS, OP_GREATER:
		binaryOp.opType = TYPE_BOOL
		if tLeft != TYPE_FLOAT && tLeft != TYPE_INT && tLeft != TYPE_STRING {
			return binaryOp, fmt.Errorf(
				"%w[%v:%v] - BinaryOp '%v' needs int/float/string, got: '%v'",
				ErrCritical, binaryOp.line, binaryOp.column, binaryOp.operator, tLeft,
			)
		}
		//return binaryOp, TYPE_BOOL, nil
	case OP_EQ, OP_NE:
		binaryOp.opType = TYPE_BOOL
		// We can actually compare all data types. So there will be no missmatch in general!
	default:
		return binaryOp, fmt.Errorf(
			"%w[%v:%v] - Invalid binary operator: '%v' for type '%v'",
			ErrCritical, binaryOp.line, binaryOp.column, binaryOp.operator, tLeft,
		)
	}

	return binaryOp, nil
}

func analyzeExpression(expression Expression, symbolTable *SymbolTable) (Expression, error) {

	switch e := expression.(type) {
	case Constant:
		return e, nil
	case Variable:

		// Lookup variable type and annotate node.
		if vTable, ok := symbolTable.getVar(e.vName); ok {
			e.vType = vTable.sType
		} else {
			return e, fmt.Errorf("%w[%v:%v] - Variable '%v' referenced before declaration", ErrCritical, e.line, e.column, e.vName)
		}
		// Always access the very last entry for variables!
		return e, nil
	case UnaryOp:
		return analyzeUnaryOp(e, symbolTable)
	case BinaryOp:
		return analyzeBinaryOp(e, symbolTable)
	}
	row, col := expression.startPos()
	return expression, fmt.Errorf("%w[%v:%v] - Unknown type for expression %v", ErrCritical, row, col, expression)
}

// Returns newly created variables and variables that should shadow others!
// This is just for housekeeping and removing them later!!!!
// All new variables (and shadow ones) are updated/written to the symbol varTable
func analyzeAssignment(assignment Assignment, symbolTable *SymbolTable) (Assignment, error) {

	// Populate/overwrite the dictionary of variables for futher statements :)
	if len(assignment.variables) != len(assignment.expressions) {

		row, col := assignment.startPos()
		if len(assignment.variables) > 0 {
			row, col = assignment.variables[0].line, assignment.variables[0].column
		}

		return assignment, fmt.Errorf(
			"%w[%v:%v] - Assignment %v - variables and expression count need to match",
			ErrCritical, row, col, assignment,
		)
	}

	for i, v := range assignment.variables {

		expression, err := analyzeExpression(assignment.expressions[i], symbolTable)
		if err != nil {
			return assignment, err
		}
		expressionType := expression.getExpressionType()

		// Shadowing is only allowed in a different block, not right after the first variable, to avoid confusion and complicated
		// variable handling
		if symbolTable.isLocalVar(v.vName) && v.vShadow {
			return assignment, fmt.Errorf(
				"%w[%v:%v] - Variable %v is shadowing another variable in the same block. This is not allowed",
				ErrCritical, v.line, v.column, v.vName,
			)
		}

		// Only, if the variable already exists and we're not trying to shadow it!
		if vTable, ok := symbolTable.getVar(v.vName); ok {
			if !v.vShadow {
				if vTable.sType != expressionType {
					return assignment, fmt.Errorf(
						"%w[%v:%v] - Assignment type missmatch between variable %v and expression %v",
						ErrCritical, v.line, v.column, v, expressionType,
					)
				}
			} else {
				symbolTable.setVar(v.vName, expressionType)
			}
		} else {
			symbolTable.setVar(v.vName, expressionType)
		}

		assignment.expressions[i] = expression

		assignment.variables[i].vType = expressionType
	}
	return assignment, nil
}

func analyzeCondition(condition Condition, symbolTable *SymbolTable) (Condition, error) {

	// This expression MUST come out as boolean!
	e, err := analyzeExpression(condition.expression, symbolTable)
	if err != nil {
		return condition, err
	}
	if e.getExpressionType() != TYPE_BOOL {
		row, col := e.startPos()
		return condition, fmt.Errorf(
			"%w[%v:%v] - If expression expected boolean, got: %v --> <<%v>>",
			ErrCritical, row, col, e.getExpressionType(), condition.expression,
		)
	}
	condition.expression = e

	block, err := analyzeBlock(condition.block, symbolTable, nil)
	if err != nil {
		return condition, err
	}
	condition.block = block

	elseBlock, err := analyzeBlock(condition.elseBlock, symbolTable, nil)
	if err != nil {
		return condition, err
	}
	condition.elseBlock = elseBlock

	return condition, nil
}

func analyzeLoop(loop Loop, symbolTable *SymbolTable) (Loop, error) {

	nextSymbolTable := SymbolTable{
		make(map[string]SymbolVarEntry, 0),
		make(map[string]SymbolFunEntry, 0),
		symbolTable,
	}

	assignment, err := analyzeAssignment(loop.assignment, &nextSymbolTable)
	if err != nil {
		return loop, err
	}
	loop.assignment = assignment

	for i, e := range loop.expressions {
		expression, err := analyzeExpression(e, &nextSymbolTable)
		if err != nil {
			return loop, err
		}
		if expression.getExpressionType() != TYPE_BOOL {
			row, col := expression.startPos()
			return loop, fmt.Errorf(
				"%w[%v:%v] - Loop expression expected boolean, got: %v (%v)",
				ErrCritical, row, col, expression.getExpressionType(), e,
			)
		}

		loop.expressions[i] = expression
	}

	incrAssignment, err := analyzeAssignment(loop.incrAssignment, &nextSymbolTable)
	if err != nil {
		return loop, err
	}
	loop.incrAssignment = incrAssignment

	statements, err := analyzeBlock(loop.block, symbolTable, &nextSymbolTable)
	if err != nil {
		return loop, err
	}
	loop.block = statements
	loop.block.symbolTable = nextSymbolTable

	return loop, nil
}

func analyzeFunction(fun Function, symbolTable *SymbolTable) (Function, error) {

	functionSymbolTable := SymbolTable{
		make(map[string]SymbolVarEntry, 0),
		make(map[string]SymbolFunEntry, 0),
		symbolTable,
	}

	for _, v := range fun.parameters {
		if v.vType == TYPE_UNKNOWN {
			return fun, fmt.Errorf("%w[%v:%v] - Function parameter %v has invalid type", ErrCritical, v.line, v.column, v)
		}
		functionSymbolTable.setVar(v.vName, v.vType)
	}

	if len(fun.returnTypes) != len(fun.returns) {
		row, col := fun.startPos()
		if len(fun.returns) > 0 {
			row, col = fun.returns[0].startPos()
		}
		return fun, fmt.Errorf("%w[%v:%v] - Function return count does not match function definition", ErrCritical, row, col)
	}

	if symbolTable.isLocalFun(fun.fName) {
		return fun, fmt.Errorf("%w[%v:%v] - Function with the same name already exists in this scope", ErrCritical, fun.line, fun.column)
	}
	var paramTypes []Type
	for _, v := range fun.parameters {
		paramTypes = append(paramTypes, v.vType)
	}
	symbolTable.setFun(fun.fName, paramTypes, fun.returnTypes)

	newBlock, err := analyzeBlock(fun.block, symbolTable, &functionSymbolTable)
	if err != nil {
		return fun, err
	}
	fun.block = newBlock

	for i, e := range fun.returns {

		newE, err := analyzeExpression(e, &functionSymbolTable)
		if err != nil {
			return fun, err
		}
		if newE.getExpressionType() != fun.returnTypes[i] {
			row, col := e.startPos()
			return fun, fmt.Errorf("%w[%v:%v] - Function return type des not match function definition", ErrCritical, row, col)
		}
		fun.returns[i] = newE
	}

	return fun, nil
}

func analyzeStatement(statement Statement, symbolTable *SymbolTable) (Statement, error) {
	switch st := statement.(type) {
	case Condition:
		return analyzeCondition(st, symbolTable)
	case Loop:
		return analyzeLoop(st, symbolTable)
	case Assignment:
		return analyzeAssignment(st, symbolTable)
	case Function:
		return analyzeFunction(st, symbolTable)
	}
	row, col := statement.startPos()
	return statement, fmt.Errorf("%w[%v:%v] - Unexpected statement: %v", ErrCritical, row, col, statement)
}

// analyzeBlock gets a reference to the current (now parent) symbol varTable
// Additionally, it might get a pre-filled symbol varTable for the new scope to use!
// This might be the case for function arguments or in a for-loop, where variables belong to the
// coming block only but are parsed in the TreeNode before.
func analyzeBlock(block Block, symbolTable, newBlockSymbolTable *SymbolTable) (Block, error) {

	if newBlockSymbolTable != nil {
		block.symbolTable = *newBlockSymbolTable
	} else {
		block.symbolTable = SymbolTable{
			make(map[string]SymbolVarEntry, 0),
			make(map[string]SymbolFunEntry, 0),
			symbolTable,
		}
	}

	for i, s := range block.statements {
		statement, err := analyzeStatement(s, &block.symbolTable)
		if err != nil {
			return block, err
		}
		block.statements[i] = statement
	}

	return block, nil
}

// analyzeTypes traverses the tree and analyzes variables with their corresponding type recursively from expressions!
// returns an error if we have a type missmatch anywhere!
func semanticAnalysis(ast AST) (AST, error) {

	ast.globalSymbolTable = SymbolTable{
		make(map[string]SymbolVarEntry, 0),
		make(map[string]SymbolFunEntry, 0),
		nil,
	}

	// TODO: Possibly fill global symbol varTable with something?
	// Right now it will stay empty just because the block we parse will create its own symbol varTable.

	block, err := analyzeBlock(ast.block, &ast.globalSymbolTable, nil)
	if err != nil {
		ast.globalSymbolTable = SymbolTable{}
		return ast, err
	}
	ast.block = block

	return ast, nil
}
