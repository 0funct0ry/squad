# Run from the repo root:
#   cp examples/ecommerce.db /tmp/cli-demo-basics.db
#   make build
#   VHS_CHROME_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
#     vhs site/scripts/vhs/cli-basics.tape
Output "site/public/casts/cli-basics.gif"

Require bash

Set Shell "bash"
Set FontSize 18
Set Width 1100
Set Height 620
Set Padding 20
Set Theme "Dracula"
Set TypingSpeed 45ms

Type "./bin/squad cli /tmp/cli-demo-basics.db"
Enter
Sleep 1.5s

Type ".tables"
Enter
Sleep 1.5s

Type ".schema products"
Enter
Sleep 2s

Type "select name, price from products order by price desc limit 5;"
Enter
Sleep 2s

Type ".mode markdown"
Enter
Sleep 500ms

Type "select name, price from products order by price desc limit 5;"
Enter
Sleep 2.5s

Type ".quit"
Enter
Sleep 1s
