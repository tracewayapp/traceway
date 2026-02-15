package services

import "testing"

func TestExtractFunctionName(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		line     int
		expected string
	}{
		{
			name: "arrow function above throw",
			content: `import React from 'react';
const handleEventError = () => {
  try {
    log("Triggering error...")
    throw new Error("Test error")
  } catch (e) {
    throw e
  }
}`,
			line:     5,
			expected: "handleEventError",
		},
		{
			name: "regular function declaration",
			content: `function processData(input) {
  transform(input)
  return input
}`,
			line:     3,
			expected: "processData",
		},
		{
			name: "export function",
			content: `export function handleError(err) {
  console.error(err)
  reportError(err)
}`,
			line:     3,
			expected: "handleError",
		},
		{
			name: "export default function",
			content: `export default function App() {
  return <div>Hello</div>
}`,
			line:     2,
			expected: "App",
		},
		{
			name: "class method",
			content: `class MyComponent {
  handleClick(event) {
    event.preventDefault()
    this.setState({ clicked: true })
  }
}`,
			line:     4,
			expected: "handleClick",
		},
		{
			name: "async method",
			content: `class ApiClient {
  async fetchData(url) {
    await fetch(url)
    return null
  }
}`,
			line:     4,
			expected: "fetchData",
		},
		{
			name: "skips function calls",
			content: `const fn = () => {
  log("hello")
  doSomething()
  throw new Error("fail")
}`,
			line:     4,
			expected: "fn",
		},
		{
			name: "skips control flow keywords",
			content: `function foo() {
  if (x) {
    doSomething()
  }
}`,
			line:     3,
			expected: "foo",
		},
		{
			name: "returns empty when no function found",
			content: `x = 1
y = 2
z = x + y`,
			line:     3,
			expected: "",
		},
		{
			name: "var declaration with function expression",
			content: `var onSubmit = function(e) {
  e.preventDefault()
  submit(data)
}`,
			line:     3,
			expected: "onSubmit",
		},
		{
			name: "let declaration with arrow",
			content: `let compute = (a, b) => {
  return a + b
}`,
			line:     2,
			expected: "compute",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFunctionName(tt.content, tt.line)
			if got != tt.expected {
				t.Errorf("extractFunctionName() = %q, want %q", got, tt.expected)
			}
		})
	}
}
