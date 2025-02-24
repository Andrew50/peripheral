#!/bin/bash

# Check if input file is provided
if [ "$#" -ne 1 ]; then
    echo "Usage: $0 script_list.txt"
    exit 1
fi

INPUT_FILE="$1"
OUTPUT_FILE="llm_context.txt"

# Check if input file exists
if [ ! -f "$INPUT_FILE" ]; then
    echo "Error: Input file '$INPUT_FILE' not found"
    exit 1
fi

# Clear or create output file
> "$OUTPUT_FILE"

# Add initial headers
echo "# Inputs" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "## Current File" >> "$OUTPUT_FILE"
echo "Here is the file I'm looking at. It might be truncated from above and below and, if so, is centered around my cursor." >> "$OUTPUT_FILE"

# Read script names from input file and add them as a list first
while IFS= read -r script_name; do
    # Skip empty lines and comments
    [[ -z "$script_name" || "$script_name" =~ ^#.*$ ]] && continue
    echo "\`\`\`${script_name}" >> "$OUTPUT_FILE"
    echo "" >> "$OUTPUT_FILE"
    echo "" >> "$OUTPUT_FILE"
done < "$INPUT_FILE"

echo "" >> "$OUTPUT_FILE"
echo "<potential_codebase_context>" >> "$OUTPUT_FILE"
echo "## Potentially Relevant Code Snippets from the current Codebase" >> "$OUTPUT_FILE"

# Now add the actual file contents
while IFS= read -r script_name; do
    # Skip empty lines and comments
    [[ -z "$script_name" || "$script_name" =~ ^#.*$ ]] && continue
    
    echo "Processing: $script_name" >&2
    
    # Use fzf to find the closest matching file
    closest_match=$(find .. -type f -print0 | fzf --read0 --print0 --query="$script_name" --select-1 --exit-0 | tr -d '\0')
    
    if [ -n "$closest_match" ]; then
        # Get relative path
        rel_path="${closest_match#../}"
        
        echo "" >> "$OUTPUT_FILE"
        echo "<file>${rel_path}</file>" >> "$OUTPUT_FILE"
        cat "$closest_match" >> "$OUTPUT_FILE"
        echo "" >> "$OUTPUT_FILE"
    else
        echo "No match found for: $script_name" >&2
    fi
    
done < "$INPUT_FILE"

echo "" >> "$OUTPUT_FILE"
echo "</potential_codebase_context>" >> "$OUTPUT_FILE"

echo "Context file generated as $OUTPUT_FILE"