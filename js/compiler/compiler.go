/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package compiler

import (
	_ "embed" // we need this for embedding Babel
	"encoding/json"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja/parser"
	"github.com/sirupsen/logrus"

	"go.k6.io/k6/lib"
)

//go:embed lib/babel.min.js
var babelSrc string //nolint:gochecknoglobals

var (
	DefaultOpts = map[string]interface{}{
		// "presets": []string{"latest"},
		"plugins": []interface{}{
			// es2015 https://github.com/babel/babel/blob/v6.26.0/packages/babel-preset-es2015/src/index.js
			[]interface{}{"transform-es2015-template-literals", map[string]interface{}{"loose": false, "spec": false}},
			"transform-es2015-literals",
			// "transform-es2015-function-name", // in goja
			// []interface{}{"transform-es2015-arrow-functions", map[string]interface{}{"spec": false}}, // in goja
			// "transform-es2015-block-scoped-functions", // in goja
			[]interface{}{"transform-es2015-classes", map[string]interface{}{"loose": false}},
			"transform-es2015-object-super",
			// "transform-es2015-shorthand-properties", // in goja
			// "transform-es2015-duplicate-keys", // in goja
			// []interface{}{"transform-es2015-computed-properties", map[string]interface{}{"loose": false}}, // in goja
			// "transform-es2015-for-of", // in goja
			// "transform-es2015-sticky-regex", // in goja
			// "transform-es2015-unicode-regex", // in goja
			// "check-es2015-constants", // in goja
			// []interface{}{"transform-es2015-spread", map[string]interface{}{"loose": false}}, // in goja
			// "transform-es2015-parameters", // in goja
			// []interface{}{"transform-es2015-destructuring", map[string]interface{}{"loose": false}}, // in goja
			// "transform-es2015-block-scoping", // in goja
			// "transform-es2015-typeof-symbol", // in goja
			// all the other module plugins are just dropped
			[]interface{}{"transform-es2015-modules-commonjs", map[string]interface{}{"loose": false}},
			// "transform-regenerator", // Doesn't really work unless regeneratorRuntime is also added

			// es2016 https://github.com/babel/babel/blob/v6.26.0/packages/babel-preset-es2016/src/index.js
			"transform-exponentiation-operator",

			// es2017 https://github.com/babel/babel/blob/v6.26.0/packages/babel-preset-es2017/src/index.js
			// "syntax-trailing-function-commas", // in goja
			// "transform-async-to-generator", // Doesn't really work unless regeneratorRuntime is also added
		},
		"ast":           false,
		"sourceMaps":    false,
		"babelrc":       false,
		"compact":       false,
		"retainLines":   true,
		"highlightCode": false,
	}

	onceBabelCode      sync.Once     // nolint:gochecknoglobals
	globalBabelCode    *goja.Program // nolint:gochecknoglobals
	globalBabelCodeErr error         // nolint:gochecknoglobals
	onceBabel          sync.Once     // nolint:gochecknoglobals
	globalBabel        *babel        // nolint:gochecknoglobals
)

const sourceMapURLFromBabel = "k6://internal-should-not-leak/file.map"

// A Compiler compiles JavaScript source code (ES5.1 or ES6) into a goja.Program
type Compiler struct {
	logger logrus.FieldLogger
	babel  *babel
	COpts  CompilerOptions // TODO change this, this is just way faster
}

// New returns a new Compiler
func New(logger logrus.FieldLogger) *Compiler {
	return &Compiler{logger: logger}
}

// initializeBabel initializes a separate (non-global) instance of babel specifically for this Compiler.
// An error is returned only if babel itself couldn't be parsed/run which should never be possible.
func (c *Compiler) initializeBabel() error {
	var err error
	if c.babel == nil {
		c.babel, err = newBabel()
	}
	return err
}

// Transform the given code into ES5
func (c *Compiler) Transform(src, filename string, inputSrcMap []byte) (code string, srcmap []byte, err error) {
	if c.babel == nil {
		onceBabel.Do(func() {
			globalBabel, err = newBabel()
		})
		c.babel = globalBabel
	}
	if err != nil {
		return
	}

	code, srcmap, err = c.babel.transformImpl(c.logger, src, filename, c.COpts.SourceMapEnabled, inputSrcMap)
	// fmt.Println(code)
	return
}

// CompilerOptions are options to the compiler ;)
type CompilerOptions struct { // TODO maybe have the fields an exported and use the functional options pattern
	CompatibilityMode lib.CompatibilityMode
	SourceMapEnabled  bool
	// TODO maybe move only this in the compiler itself and leave ht rest as parameters to the Compile
	SourceMapLoader func(string) ([]byte, error)
	Strict          bool
}

// Compile the program in the given CompatibilityMode, wrapping it between pre and post code
func (c *Compiler) Compile(src, filename string, main bool, cOpts CompilerOptions) (*goja.Program, string, error) {
	return c.compileImpl(src, filename, main, cOpts, nil)
}

func (c *Compiler) compileImpl(
	src, filename string, main bool, cOpts CompilerOptions, srcmap []byte,
) (*goja.Program, string, error) {
	code := src
	if !main {
		if len(srcmap) != 0 {
			var err error
			srcmap, err = increaseMappingsByOne(srcmap)
			if err != nil {
				return nil, "", err
			}
		}

		code = "(function(module, exports){\n" + code + "\n})\n"
	}
	opts := parser.WithDisableSourceMaps
	var couldntLoadSourceMap bool
	if cOpts.SourceMapEnabled {
		opts = parser.WithSourceMapLoader(func(path string) ([]byte, error) {
			if path == sourceMapURLFromBabel {
				return srcmap, nil
			}
			var err error
			srcmap, err = c.COpts.SourceMapLoader(path)
			if err == nil && !main {
				srcmap, err = increaseMappingsByOne(srcmap)
			} else {
				couldntLoadSourceMap = true
			}
			return srcmap, err
		})
	}
	ast, err := parser.ParseFile(nil, filename, code, 0, opts)
	// we probably don't want to abort scripts which have source maps but they can't be found,
	// this also will be a breaking change, so if we couldn't we retry with it disabled
	if couldntLoadSourceMap {
		// original error is currently not very relevant
		c.logger.Warnf("Couldn't load source map for %s", filename)
		ast, err = parser.ParseFile(nil, filename, code, 0, parser.WithDisableSourceMaps)
	}
	if err != nil {
		if cOpts.CompatibilityMode == lib.CompatibilityModeExtended {
			code, srcmap, err = c.Transform(src, filename, srcmap)
			if err != nil {
				return nil, code, err
			}
			// the compatibility mode "decreases" here as we shouldn't transform twice
			cOpts.CompatibilityMode = lib.CompatibilityModeBase
			return c.compileImpl(code, filename, main, cOpts, srcmap)
		}
		return nil, code, err
	}
	pgm, err := goja.CompileAST(ast, cOpts.Strict)
	return pgm, code, err
}

type babel struct {
	vm        *goja.Runtime
	this      goja.Value
	transform goja.Callable
	m         sync.Mutex
}

func newBabel() (*babel, error) {
	onceBabelCode.Do(func() {
		globalBabelCode, globalBabelCodeErr = goja.Compile("<internal/k6/compiler/lib/babel.min.js>", babelSrc, false)
	})
	if globalBabelCodeErr != nil {
		return nil, globalBabelCodeErr
	}
	vm := goja.New()
	_, err := vm.RunProgram(globalBabelCode)
	if err != nil {
		return nil, err
	}

	this := vm.Get("Babel")
	bObj := this.ToObject(vm)
	result := &babel{vm: vm, this: this}
	if err = vm.ExportTo(bObj.Get("transform"), &result.transform); err != nil {
		return nil, err
	}

	return result, err
}

func increaseMappingsByOne(sourceMap []byte) ([]byte, error) {
	var err error
	m := make(map[string]interface{})
	if err = json.Unmarshal(sourceMap, &m); err != nil {
		return nil, err
	}

	// ';' is the separator between lines so just adding 1 will make all mappings be for the line after which they were
	// originally
	m["mappings"] = ";" + m["mappings"].(string)
	return json.Marshal(m)
}

// transformImpl the given code into ES5, while synchronizing to ensure only a single
// bundle instance / Goja VM is in use at a time.
func (b *babel) transformImpl(
	logger logrus.FieldLogger, src, filename string, sourceMapsEnabled bool, inputSrcMap []byte,
) (string, []byte, error) {
	b.m.Lock()
	defer b.m.Unlock()
	opts := make(map[string]interface{})
	for k, v := range DefaultOpts {
		opts[k] = v
	}
	if sourceMapsEnabled {
		opts["sourceMaps"] = true
		if inputSrcMap != nil {
			srcMap := new(map[string]interface{})
			if err := json.Unmarshal(inputSrcMap, &srcMap); err != nil {
				return "", nil, err
			}
			opts["inputSourceMap"] = srcMap
		}
	}
	opts["filename"] = filename

	startTime := time.Now()
	v, err := b.transform(b.this, b.vm.ToValue(src), b.vm.ToValue(opts))
	if err != nil {
		return "", nil, err
	}
	logger.WithField("t", time.Since(startTime)).Debug("Babel: Transformed")

	vO := v.ToObject(b.vm)
	var code string
	if err = b.vm.ExportTo(vO.Get("code"), &code); err != nil {
		return code, nil, err
	}
	if !sourceMapsEnabled {
		return code, nil, nil
	}

	// this is to make goja try to load a sourcemap.
	// it is specifically a special url as it should never leak outside of this code
	// additionally the alternative support from babel is to embed *the whole* sourcemap at the end
	code += "\n//# sourceMappingURL=" + sourceMapURLFromBabel
	stringify, err := b.vm.RunString("(function(m) { return JSON.stringify(m)})")
	if err != nil {
		return code, nil, err
	}
	c, _ := goja.AssertFunction(stringify)
	mapAsJSON, err := c(goja.Undefined(), vO.Get("map"))
	if err != nil {
		return code, nil, err
	}
	return code, []byte(mapAsJSON.String()), nil
}

// Pool is a pool of compilers so it can be used easier in parallel tests as they have their own babel.
type Pool struct {
	c chan *Compiler
}

// NewPool creates a Pool that will be using the provided logger and will preallocate (in parallel)
// the count of compilers each with their own babel.
func NewPool(logger logrus.FieldLogger, count int) *Pool {
	c := &Pool{
		c: make(chan *Compiler, count),
	}
	go func() {
		for i := 0; i < count; i++ {
			go func() {
				co := New(logger)
				err := co.initializeBabel()
				if err != nil {
					panic(err)
				}
				c.Put(co)
			}()
		}
	}()

	return c
}

// Get a compiler from the pool.
func (c *Pool) Get() *Compiler {
	return <-c.c
}

// Put a compiler back in the pool.
func (c *Pool) Put(co *Compiler) {
	c.c <- co
}
