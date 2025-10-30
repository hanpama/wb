# Test Flow 3: Form Interaction (Comprehensive Input Types)

## Purpose
Test all form input types and verify diff rendering works correctly for each type.

## Endpoint
`http://localhost:8080/form`

## Form Elements to Test
1. **Text Input** - Standard text field
2. **Email Input** - Email-specific field
3. **Textarea** - Multi-line text input
4. **Select Dropdown** - Country selection
5. **Checkbox** - Newsletter subscription
6. **Radio Buttons** - Gender selection

## Expected Behavior
- All input interactions should show diff render (URL stays same)
- Each input type should display its current value/state correctly
- Diff should show only the changed elements

## Test Steps

### Setup

1. Start testserver if not running:
```bash
cd testserver && go run main.go
```

2. Open the form page:
```bash
./wb open http://localhost:8080/form
```

### Test 1: Text Input

3. Find the username text input:
```bash
./wb show | grep "username"
```

Expected output: `[Input/text: (empty)]{HASH}`

4. Fill in username:
```bash
./wb input HASH "johndoe"
```

Expected: Diff shows change from empty to "johndoe"

### Test 2: Email Input

5. Find the email input:
```bash
./wb show | grep "email"
```

Expected output: `[Input/email: (empty)]{HASH}`

6. Fill in email:
```bash
./wb input HASH "john@example.com"
```

Expected: Diff shows email value change

### Test 3: Textarea

7. Find the bio textarea:
```bash
./wb show | grep "bio"
```

Expected output: `[Input/text: (Tell us about yourself)]{HASH}` or similar

8. Fill in bio:
```bash
./wb input HASH "I am a software developer"
```

Expected: Diff shows textarea content change

### Test 4: Select Dropdown

9. Find the country select:
```bash
./wb show | grep "country"
```

Expected: Should show select element (implementation may vary)

10. Describe the select element:
```bash
./wb describe HASH
```

Expected: Should show element type as "select"

11. Click to interact with select:
```bash
./wb click HASH
```

Expected: Diff render (note: select interaction may be limited in current implementation)

### Test 5: Checkbox

12. Find the newsletter checkbox:
```bash
./wb show | grep "newsletter"
```

Expected: Checkbox element should be visible

13. Click checkbox to toggle:
```bash
./wb click HASH
```

Expected: Diff shows checkbox state change (unchecked → checked or vice versa)

14. Click again to toggle back:
```bash
./wb click HASH
```

Expected: Diff shows checkbox state change again

### Test 6: Radio Buttons

15. Find the gender radio buttons:
```bash
./wb show | grep -i "male\|female\|other"
```

Expected: Should show multiple radio button options

16. Click one radio button:
```bash
./wb click HASH_MALE
```

Expected: Diff shows radio button selection

17. Click another radio button:
```bash
./wb click HASH_FEMALE
```

Expected: Diff shows radio button change (Male unchecked, Female checked)

### Test 7: Form Submission

18. Find and click the Submit button:
```bash
./wb show | grep "Submit"
```

19. Click submit:
```bash
./wb click SUBMIT_HASH
```

Expected:
- URL stays the same (POST to same page)
- Diff shows success message
- Form values preserved in inputs

## Expected Outputs

### Text Input Diff
```
@@ -X,Y +X,Y @@
-[Input/text: (empty)]{hash}
+[Input/text: "johndoe"]{hash}
```

### Checkbox Toggle Diff
```
@@ -X,Y +X,Y @@
-Subscribe to newsletter
+[✓] Subscribe to newsletter
```

### Radio Button Diff
```
@@ -X,Y +X,Y @@
-( ) Male
-( ) Female
+(•) Male
+( ) Female
```

## Success Criteria
✅ All input types are recognized as interactive elements
✅ Text and email inputs accept typed values correctly
✅ Textarea accepts multi-line input
✅ Select dropdown is clickable (even if options aren't selectable yet)
✅ Checkbox toggles between checked/unchecked states
✅ Radio buttons show mutual exclusivity (only one selected)
✅ Form submission shows diff render (not full page reload)
✅ All interactions maintain URL (no navigation)
✅ Diff output shows only changed elements

## Known Limitations

Current implementation may have limitations for:
- **Select dropdowns**: Opening dropdown and selecting options may require additional work
- **Radio button rendering**: Visual indication of selected state depends on renderer
- **Checkbox rendering**: Visual indication of checked state depends on renderer

These limitations should be documented and addressed in future iterations.

## Notes

- Use `./wb describe HASH` to inspect element properties
- Check `Interactive` field type in describe output
- Verify that backend correctly captures element states (checked, selected, value)
