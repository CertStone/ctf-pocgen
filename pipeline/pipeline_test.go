package pipeline

import "testing"

// TestResolveProjectName 覆盖默认项目名解析的各种情况，
// 包括审查发现的越界 panic 回归（短文件名 < 4 字符）。
func TestResolveProjectName(t *testing.T) {
	cases := []struct {
		name        string
		jarPath     string
		projectName string
		want        string
	}{
		{"显式项目名优先", "/x/y/challenge.jar", "my-poc", "my-poc"},
		{"jar 后缀剥离", "/x/y/challenge.jar", "", "poc-challenge"},
		{"war 后缀剥离", "/x/y/app.war", "", "poc-app"},
		{"ear 后缀剥离", "/x/y/app.ear", "", "poc-app"},
		{"大写后缀", "/x/y/App.JAR", "", "poc-App"},
		{"无后缀", "/x/y/challenge", "", "poc-challenge"},
		// 审查发现的真实 panic 回归：极短文件名（曾触发 slice bounds out of range）
		{"单字符文件名", "/x/a", "", "poc-a"},
		{"两字符文件名", "/x/ab", "", "poc-ab"},
		{"三字符文件名", "/x/abc", "", "poc-abc"},
		// 短到带后缀也不够长的情况
		{"短 jar", "/x/a.jar", "", "poc-a"},
	}
	for _, c := range cases {
		got := ResolveProjectName(c.jarPath, c.projectName)
		if got != c.want {
			t.Errorf("%s: ResolveProjectName(%q, %q) = %q, want %q",
				c.name, c.jarPath, c.projectName, got, c.want)
		}
	}
}
