package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// HandleTestErrors serves the test page that generates JavaScript errors
func HandleTestErrors(c *fiber.Ctx) error {
    html := `
    <html>
        <head>
            <style>
                .log { margin: 10px; padding: 10px; border: 1px solid #ccc; }
                .error { color: red; }
            </style>
        </head>
        <body>
            <h1>Testing Errors</h1>
            <div id="logs"></div>
            <script>
                function addLog(msg, isError = false) {
                    const div = document.createElement('div');
                    div.className = 'log' + (isError ? ' error' : '');
                    div.textContent = msg;
                    document.getElementById('logs').appendChild(div);
                }

                // Log start
                addLog('Starting error tests...');
                
                // Network requests
                addLog('Making network requests...');
                fetch('https://api.example.com/nonexistent')
                    .catch(e => addLog('Network error: ' + e.message, true));
                
                // Reference error
                addLog('Triggering reference error...');
                try {
                    undefinedVariable.someMethod();
                } catch(e) {
                    addLog('Reference error: ' + e.message, true);
                    console.error(e);
                }
                
                // Syntax error
                addLog('Triggering syntax error...');
                try {
                    eval('if true { console.log("bad syntax") }');
                } catch(e) {
                    addLog('Syntax error: ' + e.message, true);
                    console.error(e);
                }
                
                // Type error
                addLog('Triggering type error...');
                try {
                    null.toString();
                } catch(e) {
                    addLog('Type error: ' + e.message, true);
                    console.error(e);
                }

                // Custom error
                addLog('Throwing custom error...');
                throw new Error("This is a test error");
            </script>
        </body>
    </html>
    `
    return c.Type("html").SendString(html)
} 