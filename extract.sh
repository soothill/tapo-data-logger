```bash
#!/bin/bash
awk '/^FILE: / {
    if (file) close(file)
    file = $2
    getline
    next
}
/^====/ { next }
/^END OF PROJECT ARCHIVE/ { exit }
file { print > file }
' tapo-project-complete.txt

chmod +x find-subnet.sh
echo "Files extracted successfully!"
```
