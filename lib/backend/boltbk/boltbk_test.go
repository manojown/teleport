/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package boltbk

import (
	"testing"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"

	. "gopkg.in/check.v1"
)

func TestBolt(t *testing.T) { TestingT(t) }

type BoltSuite struct {
	bk    backend.Backend
	suite test.BackendSuite
}

var _ = Suite(&BoltSuite{})

func (s *BoltSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

func (s *BoltSuite) SetUpTest(c *C) {
	var err error

	dir := c.MkDir()
	s.bk, err = New(backend.Params{
		"path": dir,
	})
	c.Assert(err, IsNil)
	c.Assert(s.bk, NotNil)

	s.suite.ChangesC = make(chan interface{})
	s.suite.B = s.bk
}

func (s *BoltSuite) TearDownTest(c *C) {
	c.Assert(s.bk.Close(), IsNil)
}

func (s *BoltSuite) TestBasicCRUD(c *C) {
	s.suite.BasicCRUD(c)
}

func (s *BoltSuite) TestBatchCRUD(c *C) {
	s.suite.BatchCRUD(c)
}

func (s *BoltSuite) TestCompareAndSwap(c *C) {
	s.suite.CompareAndSwap(c)
}

func (s *BoltSuite) TestExpiration(c *C) {
	s.suite.Expiration(c)
}

func (s *BoltSuite) TestLock(c *C) {
	s.suite.Locking(c)
}

func (s *BoltSuite) TestValueAndTTL(c *C) {
	s.suite.ValueAndTTL(c)
}
