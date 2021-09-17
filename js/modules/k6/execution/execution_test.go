/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
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

package execution

import (
	"context"
	"fmt"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
)

func TestAbortTest(t *testing.T) { //nolint: tparallel
	t.Parallel()

	rt := goja.New()
	ctx := common.WithRuntime(context.Background(), rt)
	ctx = lib.WithState(ctx, &lib.State{})
	mii := &modulestest.InstanceCore{
		Runtime: rt,
		InitEnv: &common.InitEnvironment{},
		Ctx:     ctx,
	}
	m, ok := New().NewModuleInstance(mii).(*ModuleInstance)
	require.True(t, ok)
	require.NoError(t, rt.Set("exec", m.GetExports().Default))

	prove := func(t *testing.T, script, reason string) {
		_, err := rt.RunString(script)
		require.NotNil(t, err)
		var x *goja.InterruptedError
		assert.ErrorAs(t, err, &x)
		v, ok := x.Value().(*common.InterruptError)
		require.True(t, ok)
		require.Equal(t, v.Reason, reason)
	}

	t.Run("default reason", func(t *testing.T) { //nolint: paralleltest
		prove(t, "exec.test.abort()", common.AbortTest)
	})
	t.Run("custom reason", func(t *testing.T) { //nolint: paralleltest
		prove(t, `exec.test.abort("mayday")`, fmt.Sprintf("%s: mayday", common.AbortTest))
	})
}
