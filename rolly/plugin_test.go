package rolly

import (
	. "launchpad.net/gocheck"
	"os"
	"testing"
)

func Test(t *testing.T) { TestingT(t) }

type PluginSuite struct{}

func (ps *PluginSuite) SetUpSuite(c *C) {
	PluginBase = c.MkDir()
	os.Setenv("RB_PLUGIN_URL", "https://rollbackup.com")
}

var _ = Suite(&PluginSuite{})

func (self *PluginSuite) TestDownload(c *C) {
	p := &Plugin{Name: "mock", Version: "0.0.1"}
	c.Check(p.Exists(), Equals, false)

	err := p.Download()
	c.Assert(err, IsNil)
	c.Check(p.Exists(), Equals, true)

	dir := c.MkDir()
	params := map[string]string{
		"param": "test",
	}
	err = p.Run(dir, params)
	c.Assert(err, IsNil)

}
