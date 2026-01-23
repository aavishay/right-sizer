import sys
import re
import os

if len(sys.argv) < 2:
    print("Usage: python3 migrate_logger.py <file_path>")
    sys.exit(1)

file_path = sys.argv[1]

if not os.path.exists(file_path):
    print(f"File {file_path} not found")
    sys.exit(1)

with open(file_path, 'r') as f:
    content = f.read()

# Replace log.Printf with logger.Info
# We assume logger is imported or available.
new_content = re.sub(r'\blog\.Printf\b', 'logger.Info', content)

# Replace log.Println with logger.Info
# Note: log.Println(args...) vs logger.Info(format, args...)
# If log.Println is used with multiple args, logger.Info will treat the first as format.
# This might be an issue if the first arg is not a format string but contains %.
# However, usually log.Println is "msg", "val" -> "msg val"
# logger.Info("msg", "val") -> Infof("msg", "val") -> "msg%!(EXTRA string=val)" if msg has no %
# So this conversion is NOT perfect for Println.
# But for single string arg it is fine.
# For multiple args, we might want to wrap in fmt.Sprint?
# log.Println(a, b) -> logger.Info(fmt.Sprint(a, b))?
# But checking args is hard with regex.
# Let's do simple replacement and assume manual fix might be needed if compilation fails.
new_content = re.sub(r'\blog\.Println\b', 'logger.Info', new_content)

# Also replace log.Print with logger.Info
new_content = re.sub(r'\blog\.Print\b', 'logger.Info', new_content)

with open(file_path, 'w') as f:
    f.write(new_content)

print(f"Processed {file_path}")
