#!/bin/bash

# wb Test Runner
# This script helps run common test scenarios

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

WB="../wb"
SERVER_URL="http://localhost:8080"

# Check if wb is built
if [ ! -f "$WB" ]; then
    echo -e "${RED}Error: wb binary not found. Please build it first:${NC}"
    echo "  cd .. && go build -o wb"
    exit 1
fi

# Check if server is running
if ! curl -s "$SERVER_URL" > /dev/null 2>&1; then
    echo -e "${RED}Error: Test server is not running.${NC}"
    echo "Please start it in another terminal:"
    echo "  cd testserver && go run main.go"
    exit 1
fi

echo -e "${GREEN}wb Test Runner${NC}"
echo "================================"
echo ""

# Test menu
echo "Select a test to run:"
echo ""
echo "  1) Instant Load - No network requests"
echo "  2) Slow Resources - Network status tracking"
echo "  3) Form Interaction - Diff rendering"
echo "  4) Click Navigation - Full re-render"
echo "  5) SPA Updates - JavaScript diff rendering"
echo "  6) Multiple Requests - Concurrent request tracking"
echo "  7) Run all tests (interactive)"
echo "  0) Exit"
echo ""

read -p "Enter your choice [0-7]: " choice

run_test_1() {
    echo -e "\n${YELLOW}Test 1: Instant Load${NC}"
    echo "Opening $SERVER_URL/instant..."
    $WB open "$SERVER_URL/instant"
    echo ""
    echo "Running 'wb show' - should NOT show network status:"
    $WB show
    echo ""
    echo -e "${GREEN}✓ Check: No 'network requests in progress' footer should appear${NC}"
}

run_test_2() {
    echo -e "\n${YELLOW}Test 2: Slow Resources${NC}"
    echo "Opening $SERVER_URL/slow-resource..."
    $WB open "$SERVER_URL/slow-resource"
    echo ""
    echo "Immediately running 'wb show' - should show network requests:"
    $WB show
    echo ""
    echo "Waiting 2 seconds..."
    sleep 2
    echo "Running 'wb show' again - fewer requests:"
    $WB show
    echo ""
    echo "Waiting 4 more seconds..."
    sleep 4
    echo "Running 'wb show' again - should be stable:"
    $WB show
    echo ""
    echo -e "${GREEN}✓ Check: Request count should decrease over time${NC}"
}

run_test_3() {
    echo -e "\n${YELLOW}Test 3: Form Interaction${NC}"
    echo "Opening $SERVER_URL/form..."
    $WB open "$SERVER_URL/form"
    echo ""
    echo "Finding input field..."
    INPUT_HASH=$($WB show | grep -o 'Input/text[^}]*}' | grep -o '{[^}]*}' | tr -d '{}' | head -1)

    if [ -z "$INPUT_HASH" ]; then
        echo -e "${RED}✗ Could not find input field hash${NC}"
        return 1
    fi

    echo "Input field hash: $INPUT_HASH"
    echo ""
    echo "Typing 'testuser' into field..."
    $WB input "$INPUT_HASH" "testuser"
    echo ""
    echo -e "${GREEN}✓ Check: Should see diff output (not full render)${NC}"
    echo -e "${GREEN}✓ Check: Diff should show value change${NC}"
}

run_test_4() {
    echo -e "\n${YELLOW}Test 4: Click Navigation${NC}"
    echo "Opening $SERVER_URL/navigation?page=1..."
    $WB open "$SERVER_URL/navigation?page=1"
    echo ""
    echo "Finding Page 2 link..."
    PAGE2_HASH=$($WB show | grep 'Page 2' | grep -o '{[^}]*}' | tr -d '{}' | head -1)

    if [ -z "$PAGE2_HASH" ]; then
        echo -e "${RED}✗ Could not find Page 2 link${NC}"
        return 1
    fi

    echo "Page 2 link hash: $PAGE2_HASH"
    echo ""
    echo "Clicking to navigate to Page 2..."
    $WB click "$PAGE2_HASH"
    echo ""
    echo -e "${GREEN}✓ Check: Should see FULL render of Page 2 (not diff)${NC}"
    echo -e "${GREEN}✓ Check: URL should change to page=2${NC}"
}

run_test_5() {
    echo -e "\n${YELLOW}Test 5: SPA Updates${NC}"
    echo "Opening $SERVER_URL/spa..."
    $WB open "$SERVER_URL/spa"
    echo ""
    echo "Finding Update Content button..."
    BUTTON_HASH=$($WB show | grep 'Update Content' | grep -o '{[^}]*}' | tr -d '{}' | head -1)

    if [ -z "$BUTTON_HASH" ]; then
        echo -e "${RED}✗ Could not find button hash${NC}"
        return 1
    fi

    echo "Button hash: $BUTTON_HASH"
    echo ""
    echo "Clicking button (first time)..."
    $WB click "$BUTTON_HASH"
    echo ""
    echo "Clicking button (second time)..."
    $WB click "$BUTTON_HASH"
    echo ""
    echo -e "${GREEN}✓ Check: Should see diff output (not full render)${NC}"
    echo -e "${GREEN}✓ Check: Diff should show counter increment${NC}"
}

run_test_6() {
    echo -e "\n${YELLOW}Test 6: Multiple Concurrent Requests${NC}"
    echo "Opening $SERVER_URL/multiple-requests..."
    echo "(This will wait for initial stability)"
    $WB open "$SERVER_URL/multiple-requests"
    echo ""
    echo "Checking status immediately:"
    $WB show
    echo ""
    echo "Waiting 2 seconds..."
    sleep 2
    echo "Checking status again:"
    $WB show
    echo ""
    echo "Waiting 3 more seconds (5 total)..."
    sleep 3
    echo "Checking final status:"
    $WB show
    echo ""
    echo -e "${GREEN}✓ Check: Request count should decrease over time${NC}"
    echo -e "${GREEN}✓ Check: All requests should complete within 5 seconds${NC}"
}

case $choice in
    1)
        run_test_1
        ;;
    2)
        run_test_2
        ;;
    3)
        run_test_3
        ;;
    4)
        run_test_4
        ;;
    5)
        run_test_5
        ;;
    6)
        run_test_6
        ;;
    7)
        echo -e "\n${YELLOW}Running all tests...${NC}\n"
        run_test_1
        read -p "Press Enter to continue to next test..."
        run_test_2
        read -p "Press Enter to continue to next test..."
        run_test_3
        read -p "Press Enter to continue to next test..."
        run_test_4
        read -p "Press Enter to continue to next test..."
        run_test_5
        read -p "Press Enter to continue to next test..."
        run_test_6
        echo -e "\n${GREEN}All tests complete!${NC}"
        ;;
    0)
        echo "Exiting..."
        exit 0
        ;;
    *)
        echo -e "${RED}Invalid choice${NC}"
        exit 1
        ;;
esac

echo ""
echo "================================"
echo -e "${GREEN}Test complete!${NC}"
echo ""
echo "For detailed test flows, see:"
echo "  testflows/0X-*.md"
