package base

import (
	"fmt"
	"strconv"
	"regexp"
	//"github.com/skycoin/skycoin/src/cipher/encoder"
)

func PrintAffordances (affs []*cxAffordance) {
	for _, aff := range affs {
		fmt.Println(aff.Description)
	}
}

func (aff *cxAffordance) ApplyAffordance () {
	aff.Action()
}

func FilterAffordances(affs []*cxAffordance, filters ...string) []*cxAffordance {
	filteredAffs := make([]*cxAffordance, 0)
	for _, filter := range filters {
		//re := regexp.MustCompile(regexp.QuoteMeta(filter))
		re := regexp.MustCompile(filter)
		for _, aff := range affs {
			if re.FindString(aff.Description) != "" {
				filteredAffs = append(filteredAffs, aff)
			}
		}
		affs = filteredAffs
		filteredAffs = make([]*cxAffordance, 0)
	}
	return affs
}

func (strct *cxStruct) GetAffordances() []*cxAffordance {
	affs := make([]*cxAffordance, 0)
	mod := strct.Module

	types := make([]string, len(basicTypes))
	copy(types, basicTypes)
	
	for name, _ := range mod.Structs {
		types = append(types, name)
	}

	// Getting types from imported modules
	for impName, imp := range mod.Imports {
		for _, strct := range imp.Structs {
			types = append(types, concat(impName, ".", strct.Name))
		}
	}

	// definitions for each available type
	for _, typ := range types {
		fldGensym := MakeGenSym("fld")
		fldType := MakeType(typ)
		
		affs = append(affs, &cxAffordance{
			Description: concat("AddField ", fldGensym, " ", typ),
			Action: func() {
				strct.AddField(MakeField(fldGensym, fldType))
			}})
	}

	return affs
}

func (expr *cxExpression) GetAffordances() []*cxAffordance {
	op := expr.Operator
	affs := make([]*cxAffordance, 0)

	// The operator for this function doesn't require arguments
	if len(op.Inputs) > 0 && len(expr.Arguments) < len(op.Inputs) {
		fn := expr.Function
		mod := expr.Module
		reqType := op.Inputs[len(expr.Arguments)].Typ.Name // Required type for the current op's input
		defsTypes := make([]string, 0)
		args := make([]*cxArgument, 0)
		identType := MakeType("ident")

		inOutNames := make([]string, len(fn.Inputs) + 1)
		
		// Adding inputs and outputs as definitions
		// inputs
		for i, param := range fn.Inputs {
			if reqType == param.Typ.Name {
				inOutNames[i] = param.Name
				defsTypes = append(defsTypes, param.Typ.Name)
				identName := []byte(param.Name)
				args = append(args, &cxArgument{
					Typ: identType,
					Value: &identName,
					Offset: -1,
					Size: -1,})
			}
		}
		
		// // outputs
		// for i, param := range fn.Outputs {
		// 	if reqType == param.Typ.Name {
		// 		inOutNames[i] = param.Name
		// 		defsTypes = append(defsTypes, param.Typ.Name)
		// 		identName := []byte(param.Name)
		// 		args = append(args, &cxArgument{
		// 			Typ: identType,
		// 			Value: &identName,
		// 			Offset: -1,
		// 			Size: -1,})
		// 	}
		// }

		// Adding definitions (global vars)
		for _, def := range mod.Definitions {
			if reqType == def.Typ.Name {
				// we could have a var with the same name and type in global and local
				// contexts. We only want to show 1 affordance for this name
				notDuplicated := true
				for _, name := range inOutNames {
					if name == def.Name {
						notDuplicated = false
						break
					}
				}
				
				if notDuplicated {
					defsTypes = append(defsTypes, def.Typ.Name)
					identName := []byte(def.Name)
					args = append(args, &cxArgument{
						Typ: identType,
						Value: &identName,
						Offset: -1,
						Size: -1,})
				}
			}
		}

		// Adding local definitions
		for _, ex := range expr.Function.Expressions {
			
			if ex == expr {
				continue
			}

			if len(ex.Operator.Outputs) != len(ex.OutputNames) ||
				len(ex.Operator.Inputs) != len(ex.Arguments) {
				// Then it's not a completed expression
				continue
			}

			for i, out := range ex.Operator.Outputs {
				defsTypes = append(defsTypes, out.Typ.Name)
				identName := []byte(ex.OutputNames[i])
				args = append(args, &cxArgument{
					Typ: identType,
					Value: &identName,
					Offset: -1,
					Size: -1,
				})
			}
		}

		for i, arg := range args {
			theArg := arg
			affs = append(affs, &cxAffordance{
				Description: concat("AddArgument ", string(*arg.Value), " ", defsTypes[i]),
				Action: func() {
					expr.AddArgument(theArg)
				}})
		}
	}

	// Output names affordances
	if len(expr.OutputNames) < len(expr.Operator.Outputs) {
		outName := MakeGenSym("var")
		affs = append(affs, &cxAffordance{
			Description: concat("AddOutputName ", outName),
			Action: func() {
				expr.AddOutputName(outName)
			}})
	}

	return affs
}

func (fn *cxFunction) GetAffordances() []*cxAffordance {
	affs := make([]*cxAffordance, 0)

	for _, fnName := range basicFunctions {
		if fnName == fn.Name {
			return affs
		}
	}
	
	mod := fn.Module
	opsNames := make([]string, 0)
	ops := make([]*cxFunction, 0)

	types := make([]string, len(basicTypes))
	copy(types, basicTypes)
	for name, _ := range mod.Structs {
		types = append(types, name)
	}

	// Getting types from imported modules
	for impName, imp := range mod.Imports {
		for _, strct := range imp.Structs {
			types = append(types, concat(impName, ".", strct.Name))
		}
	}

	// Getting operators from current module
	for opName, op := range mod.Functions {
		if fn.Name != opName && opName != "main" {
			ops = append(ops, op)
			opsNames = append(opsNames, opName)
		}
	}

	// Getting operators from imported modules
	for impName, imp := range mod.Imports {
		for opName, op := range imp.Functions {
			ops = append(ops, op)
			opsNames = append(opsNames, concat(impName, ".", opName))
		}
	}

	// Inputs
	for _, typ := range types {
		affs = append(affs, &cxAffordance{
			Description: concat("AddInput ", typ),
			Action: func() {
				fn.AddInput(MakeParameter(MakeGenSym("in"), MakeType(typ)))
		}})
	}

	// Outputs
	for _, typ := range types {
		affs = append(affs, &cxAffordance{
			Description: concat("AddOutput ", typ),
			Action: func() {
				fn.AddInput(MakeParameter(MakeGenSym("in"), MakeType(typ)))
			}})
	}

	// Expressions
	for i, op := range ops {
		theOp := op // or will keep reference to last op
		affs = append(affs, &cxAffordance{
			Description: concat("AddExpression ", opsNames[i]),
			Action: func() {
				fn.AddExpression(MakeExpression(theOp))
		}})
	}

	return affs
}

// func (fn *cxFunction) GetAffordances() []*cxAffordance {
// 	affs := make([]*cxAffordance, 0)

// 	for _, fnName := range basicFunctions {
// 		if fnName == fn.Name {
// 			return affs
// 		}
// 	}
	
// 	mod := fn.Module
// 	opsNames := make([]string, 0)
// 	ops := make([]*cxFunction, 0)
// 	//defs := make([]*cxDefinition, 0)
// 	// we only need the names and all of them will be of type ident
// 	defs := make([]string, 0)
// 	defsTypes := make([]*cxType, 0)

// 	types := make([]string, len(basicTypes))
// 	copy(types, basicTypes)
// 	for name, _ := range mod.Structs {
// 		types = append(types, name)
// 	}

// 	// Getting types from imported modules
// 	for impName, imp := range mod.Imports {
// 		for _, strct := range imp.Structs {
// 			types = append(types, concat(impName, ".", strct.Name))
// 		}
// 	}

// 	// Getting operators from current module
// 	for opName, op := range mod.Functions {
// 		if fn.Name != opName && opName != "main" {
// 			ops = append(ops, op)
// 			opsNames = append(opsNames, opName)
// 		}
// 	}

// 	// Getting operators from imported modules
// 	for impName, imp := range mod.Imports {
// 		for opName, op := range imp.Functions {
// 			ops = append(ops, op)
// 			opsNames = append(opsNames, concat(impName, ".", opName))
// 		}
// 	}

// 	//Getting global definitions from current module
// 	for defName, def := range mod.Definitions {
// 		defs = append(defs, defName)
// 		defsTypes = append(defsTypes, def.Typ)
// 	}

// 	//Getting global definitions from imported modules
// 	for _, imp := range mod.Imports {
// 		for defName, def := range imp.Definitions {
// 			defs = append(defs, defName)
// 			defsTypes = append(defsTypes, def.Typ)
// 		}
// 	}

// 	// Getting input defs
// 	// We might need to create an empty definition?
// 	onlyLocals := make([]string, 0)
// 	onlyLocalsTypes := make([]string, 0)
// 	for _, inp := range fn.Inputs {
// 		defs = append(defs, inp.Name)
// 		onlyLocals = append(onlyLocals, inp.Name)
// 		onlyLocalsTypes = append(onlyLocalsTypes, inp.Typ.Name)
// 		defsTypes = append(defsTypes, inp.Typ)
// 	}

// 	// Getting output def
// 	// *why commenting it* The output definition CAN be an argument to another expr
// 	// But it SHOULD NOT be used as an argument
// 	for _, inp := range fn.Outputs {
// 		//defs = append(defs, inp.Name)
// 		onlyLocals = append(onlyLocals, inp.Name)
// 		onlyLocalsTypes = append(onlyLocalsTypes, inp.Typ.Name)
// 		//defsTypes = append(defsTypes, inp.Typ)
// 	}

// 	// Getting local definitions
// 	for _, expr := range fn.Expressions {
		


// 		for i, outName := range expr.OutputNames {
// 			cont := true
// 			for _, def := range defs {
// 				if outName == def {
// 					cont = false
// 				}
// 			}
// 			for _, out := range fn.Outputs {
// 				if outName == out.Name {
// 					cont = false
// 				}
// 			}

// 			if cont {
// 				defs = append(defs, outName)
// 				defsTypes = append(defsTypes, expr.Operator.Outputs[i].Typ)
// 				onlyLocals = append(onlyLocals, outName)
// 				onlyLocalsTypes = append(onlyLocalsTypes, expr.Operator.Outputs[i].Typ.Name)
// 			}
// 		}
// 	}

// 	// Input affs
// 	for _, typ := range types {
// 		affs = append(affs, &cxAffordance{
// 			Description: concat("AddInput ", typ),
// 			Action: func() {
// 				fn.AddInput(MakeParameter(MakeGenSym("in"), MakeType(typ)))
// 			}})
// 	}

// 	// Output affs
// 	for _, typ := range types {
// 		affs = append(affs, &cxAffordance{
// 			Description: concat("AddOutput ", typ),
// 			Action: func() {
// 				fn.AddInput(MakeParameter(MakeGenSym("out"), MakeType(typ)))
// 			}})
// 	}

// 	ident := MakeType("ident")
// 	for opIndex, op := range ops {
// 		theOp := op // or will keep reference to last op

// 		inputArgs := make([][]*cxArgument, 0)
// 		inputArgsTypes := make([][]string, 0)
// 		for _, inp := range theOp.Inputs {
// 			args := make([]*cxArgument, 0)
// 			argsTypes := make([]string, 0)
// 			for j, def := range defs {
// 				if defsTypes[j].Name == inp.Typ.Name {
// 					arg := MakeArgument(MakeValue(def), ident)
// 					//arg := MakeArgument(MakeValue(def), inp.Typ)
// 					args = append(args, arg)
// 					argsTypes = append(argsTypes, inp.Typ.Name)
// 				}
// 			}
// 			if len(args) > 0 {
// 				inputArgs = append(inputArgs, args)
// 				inputArgsTypes = append(inputArgsTypes, argsTypes)
// 			}
// 		}

// 		numberCombinations := 1
// 		for _, args := range inputArgs {
// 			numberCombinations = numberCombinations * len(args)
// 		}

// 		finalArguments := make([][]*cxArgument, numberCombinations)
// 		finalArgumentsTypes := make([][]string, numberCombinations)
// 		for i, args := range inputArgs {
// 			for j := 0; j < numberCombinations; j++ {
// 				x := 1
// 				for _, a := range inputArgs[i+1:] {
// 					x = x * len(a)
// 				}
// 				finalArguments[j] = append(finalArguments[j], args[(j / x) % len(args)])
// 				finalArgumentsTypes[j] = append(finalArgumentsTypes[j], inputArgsTypes[i][(j / x) % len(inputArgsTypes[i])])
// 			}
// 		}

// 		onlyLocals = append(onlyLocals, MakeGenSym("var"))
// 		onlyLocalsTypes = append(onlyLocalsTypes, "ident")
// 		//onlyLocals = removeDuplicates(onlyLocals)

// 		for _, args := range finalArguments {
// 			for i, local := range onlyLocals {
// 				// if a var was initialized of one type, we can't assign another type to this var later on
// 				if (onlyLocalsTypes[i] != theOp.Output.Typ.Name &&
// 					onlyLocalsTypes[i] != "ident") &&
// 					local != fn.Output.Name {
// 					continue
// 				}
				
// 				for _, out := range theOp.Outputs {
// 					if onlyLocalsTypes
// 				}


// 				// skip affordances where the operator's output type doesn't match function's output type
// 				// and we're assigning this to the function's output var
// 				if local == fn.Output.Name && theOp.Output.Typ.Name != fn.Output.Typ.Name {
// 					continue
// 				}
				
// 				varExpr := local

// 				identNames := ""
// 				//fmt.Println(args)
// 				for i, arg := range args {
// 					if i == len(args) - 1 {
// 						identNames = concat(identNames, string(*arg.Value))
// 					} else {
// 						identNames = concat(identNames, string(*arg.Value), ", ")
// 					}
					
// 				}

// 				argsCopy := make([]*cxArgument, len(args))
// 				for i, arg := range args {
// 					argsCopy[i] = MakeArgumentCopy(arg)
// 					//fmt.Println(string(*argsCopy[i].Value))
// 				}

// 				affs = append(affs, &cxAffordance{
// 					Description: fmt.Sprintf("AddExpression %s = %s(%s)", varExpr, opsNames[opIndex], identNames),
// 					Action: func() {
// 						expr := MakeExpression(varExpr, theOp)
// 						fn.AddExpression(expr)
// 						for _, arg := range argsCopy {
// 							expr.AddArgument(arg)
// 						}
// 					}})
// 			}
// 		}
// 	}
	
// 	return affs
// }

func (mod *cxModule) GetAffordances() []*cxAffordance {
	affs := make([]*cxAffordance, 0)
	types := make([]string, len(basicTypes))
	copy(types, basicTypes)

	if len(mod.Structs) > 0 {
		for name, _ := range mod.Structs {
			types = append(types, name)
		}
	}

	// Getting types from imported modules
	for impName, imp := range mod.Imports {
		for _, strct := range imp.Structs {
			types = append(types, concat(impName, ".", strct.Name))
		}
	}

	// definitions for each available type
	for _, typ := range types {
		defGensym := MakeGenSym("def")
		defType := MakeType(typ)
		value := []byte{}
		
		affs = append(affs, &cxAffordance{
			Description: concat("AddDefinition ", defGensym, " ", typ),
			Action: func() {
				mod.AddDefinition(MakeDefinition(defGensym, &value, defType))
			}})
	}

	// add imports
	for _, imp := range mod.Context.Modules {
		if imp.Name != mod.Name {
			affs = append(affs, &cxAffordance{
				Description: concat("AddImport ", imp.Name),
				Action: func() {
					mod.AddImport(imp)
				}})
		}
	}
	
	// add function
	fnGensym := MakeGenSym("fn")
	affs = append(affs, &cxAffordance{
		Description: concat("AddFunction ", fnGensym),
		Action: func() {
			mod.AddFunction(MakeFunction(fnGensym))
		}})

	// add structure
	strctGensym := MakeGenSym("strct")
	affs = append(affs, &cxAffordance{
		Description: concat("AddStruct ", strctGensym),
		Action: func() {
			mod.AddStruct(MakeStruct(strctGensym))
		}})
	
	return affs
}

func (cxt *cxContext) GetAffordances() []*cxAffordance {
	affs := make([]*cxAffordance, 0)
	modGensym := MakeGenSym("mod")
	
	affs = append(affs, &cxAffordance {
		Description: concat("AddModule ", modGensym),
		Action: func() {
			cxt.AddModule(MakeModule(modGensym))
		}})

	// Select module
	for _, mod := range cxt.Modules {
		modName := mod.Name
		affs = append(affs, &cxAffordance {
			Description: concat("SelectModule ", modName),
			Action: func() {
				cxt.SelectModule(modName)
			}})
	}

	// Select function from current module
	if cxt.CurrentModule != nil {
		for _, fn := range cxt.CurrentModule.Functions {
			fnName := fn.Name
			affs = append(affs, &cxAffordance {
				Description: concat("SelectFunction ", fnName),
				Action: func() {
					cxt.SelectFunction(fnName)
				}})
		}
	}

	// Select struct from current module
	if cxt.CurrentModule != nil {
		for _, strct := range cxt.CurrentModule.Structs {
			strctName := strct.Name
			affs = append(affs, &cxAffordance {
				Description: concat("SelectStruct ", strctName),
				Action: func() {
					cxt.SelectStruct(strctName)
				}})
		}
	}

	// Select expression from current function
	if cxt.CurrentModule != nil && cxt.CurrentModule.CurrentFunction != nil {
		for _, expr := range cxt.CurrentModule.CurrentFunction.Expressions {
			lineNumber := expr.Line
			line := strconv.Itoa(lineNumber)
			
			affs = append(affs, &cxAffordance {
				Description: fmt.Sprintf("SelectExpression (%s.%s) Line # %s", expr.Module.Name, expr.Function.Name, line),
				Action: func() {
					cxt.SelectExpression(lineNumber)
				}})
		}
	}
	
	return affs
}
