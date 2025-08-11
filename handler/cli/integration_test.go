package cli_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/elct9620/ccmon/entity"
	"github.com/elct9620/ccmon/handler/cli"
	"github.com/elct9620/ccmon/service"
	"github.com/elct9620/ccmon/testutil"
	"github.com/elct9620/ccmon/usecase"
)

// Helper function to calculate expected daily usage percentage based on current month
func calculateExpectedDailyUsage(dailyCost, planPrice float64) string {
	// Use current month days to match TimePeriodFactory behavior
	now := time.Now()
	// Get the number of days in the current month
	nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	daysInMonth := int(nextMonth.Sub(thisMonth).Hours() / 24)

	dailyBudget := planPrice / float64(daysInMonth)
	percentage := int((dailyCost / dailyBudget) * 100)
	return fmt.Sprintf("%d%%", percentage)
}

// Helper function for creating API requests - uses current date to match period factory
func createTestAPIRequests(dailyBaseRequests, dailyPremiumRequests, monthlyBaseRequests, monthlyPremiumRequests int,
	dailyBaseCost, dailyPremiumCost, monthlyBaseCost, monthlyPremiumCost float64) []entity.APIRequest {
	// Use America/New_York timezone to match the test's period factory
	timezone, _ := time.LoadLocation("America/New_York")
	return createTestAPIRequestsForCurrentDate(dailyBaseRequests, dailyPremiumRequests, monthlyBaseRequests, monthlyPremiumRequests,
		dailyBaseCost, dailyPremiumCost, monthlyBaseCost, monthlyPremiumCost, timezone)
}

// Helper function to create API requests for current date (matches TimePeriodFactory behavior)
func createTestAPIRequestsForCurrentDate(dailyBaseRequests, dailyPremiumRequests, monthlyBaseRequests, monthlyPremiumRequests int,
	dailyBaseCost, dailyPremiumCost, monthlyBaseCost, monthlyPremiumCost float64, timezone *time.Location) []entity.APIRequest {
	var requests []entity.APIRequest

	// Use current date to match TimePeriodFactory's time.Now() calls
	now := time.Now().In(timezone)
	today := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, timezone)
	monthStart := time.Date(now.Year(), now.Month(), 1, 12, 0, 0, 0, timezone)

	// Create daily base requests
	for i := 0; i < dailyBaseRequests; i++ {
		req := entity.NewAPIRequest(
			fmt.Sprintf("daily-base-%d", i),
			today,
			"claude-3-haiku-20240307",
			entity.NewToken(200, 160, 0, 0),
			entity.NewCost(dailyBaseCost/float64(dailyBaseRequests)),
			1000,
		)
		requests = append(requests, req)
	}

	// Create daily premium requests
	for i := 0; i < dailyPremiumRequests; i++ {
		req := entity.NewAPIRequest(
			fmt.Sprintf("daily-premium-%d", i),
			today,
			"claude-3-5-sonnet-20241022",
			entity.NewToken(666, 500, 0, 0),
			entity.NewCost(dailyPremiumCost/float64(dailyPremiumRequests)),
			1000,
		)
		requests = append(requests, req)
	}

	// Create monthly base requests (excluding daily ones)
	for i := 0; i < monthlyBaseRequests; i++ {
		req := entity.NewAPIRequest(
			fmt.Sprintf("monthly-base-%d", i),
			monthStart.Add(time.Duration(i)*time.Hour),
			"claude-3-haiku-20240307",
			entity.NewToken(200, 160, 0, 0),
			entity.NewCost(monthlyBaseCost/float64(monthlyBaseRequests)),
			1000,
		)
		requests = append(requests, req)
	}

	// Create monthly premium requests (excluding daily ones)
	for i := 0; i < monthlyPremiumRequests; i++ {
		req := entity.NewAPIRequest(
			fmt.Sprintf("monthly-premium-%d", i),
			monthStart.Add(time.Duration(i)*time.Hour),
			"claude-3-5-sonnet-20241022",
			entity.NewToken(666, 500, 0, 0),
			entity.NewCost(monthlyPremiumCost/float64(monthlyPremiumRequests)),
			1000,
		)
		requests = append(requests, req)
	}

	return requests
}

func TestFormatQueryEndToEnd(t *testing.T) {
	// Create timezone for testing
	timezone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("Failed to load timezone: %v", err)
	}

	tests := []struct {
		name           string
		formatString   string
		plan           entity.Plan
		planErr        error
		requests       []entity.APIRequest
		repositoryErr  error
		expectedOutput string
		expectError    bool
	}{
		{
			name:           "single daily cost variable with pro plan",
			formatString:   "@daily_cost",
			plan:           entity.NewPlan("pro", entity.NewCost(20.0)),
			requests:       createTestAPIRequests(3, 2, 10, 5, 5.0, 10.0, 50.0, 90.0),
			expectedOutput: "$15.0",
		},
		{
			name:           "single monthly cost variable",
			formatString:   "@monthly_cost",
			plan:           entity.NewPlan("pro", entity.NewCost(20.0)),
			requests:       createTestAPIRequests(3, 2, 10, 5, 5.0, 10.0, 50.0, 90.0),
			expectedOutput: "$155.0", // daily (15.0) + monthly additional (140.0) = 155.0
		},
		{
			name:           "daily plan usage with pro plan",
			formatString:   "@daily_plan_usage",
			plan:           entity.NewPlan("pro", entity.NewCost(20.0)),
			requests:       createTestAPIRequests(3, 2, 10, 5, 5.0, 10.0, 50.0, 90.0),
			expectedOutput: calculateExpectedDailyUsage(15.0, 20.0), // New formula: $15.0 / ($20.0 / days in current month)
		},
		{
			name:           "monthly plan usage with max plan",
			formatString:   "@monthly_plan_usage",
			plan:           entity.NewPlan("max", entity.NewCost(100.0)),
			requests:       createTestAPIRequests(3, 2, 10, 5, 5.0, 10.0, 50.0, 90.0),
			expectedOutput: "155%", // $155.0 / $100.0 = 155%
		},
		{
			name:           "multiple variables in format string",
			formatString:   "Daily: @daily_cost Monthly: @monthly_cost Usage: @daily_plan_usage",
			plan:           entity.NewPlan("pro", entity.NewCost(20.0)),
			requests:       createTestAPIRequests(3, 2, 10, 5, 5.0, 10.0, 50.0, 90.0),
			expectedOutput: fmt.Sprintf("Daily: $15.0 Monthly: $155.0 Usage: %s", calculateExpectedDailyUsage(15.0, 20.0)),
		},
		{
			name:           "format string with emojis and custom text",
			formatString:   "💰 Daily: @daily_cost | 📊 Monthly: @monthly_cost | 📈 @daily_plan_usage of plan",
			plan:           entity.NewPlan("pro", entity.NewCost(20.0)),
			requests:       createTestAPIRequests(3, 2, 10, 5, 5.0, 10.0, 50.0, 90.0),
			expectedOutput: fmt.Sprintf("💰 Daily: $15.0 | 📊 Monthly: $155.0 | 📈 %s of plan", calculateExpectedDailyUsage(15.0, 20.0)),
		},
		{
			name:           "unset plan returns zero percentage",
			formatString:   "@daily_plan_usage @monthly_plan_usage",
			plan:           entity.NewPlan("unset", entity.NewCost(0)),
			requests:       createTestAPIRequests(3, 2, 10, 5, 5.0, 10.0, 50.0, 90.0),
			expectedOutput: "0% 0%",
		},
		{
			name:           "plan repository error - fallback to unset",
			formatString:   "@daily_cost @daily_plan_usage",
			planErr:        fmt.Errorf("failed to get plan"),
			requests:       createTestAPIRequests(3, 2, 10, 5, 5.0, 10.0, 50.0, 90.0),
			expectedOutput: "$15.0 0%",
		},
		{
			name:           "max20 plan percentage calculation",
			formatString:   "@monthly_plan_usage",
			plan:           entity.NewPlan("max20", entity.NewCost(200.0)),
			requests:       createTestAPIRequests(3, 2, 10, 5, 5.0, 10.0, 50.0, 90.0),
			expectedOutput: "77%", // $155.0 / $200.0 = 77.5% -> 77%
		},
		{
			name:           "no requests - zero costs",
			formatString:   "@daily_cost @monthly_cost @daily_plan_usage",
			plan:           entity.NewPlan("pro", entity.NewCost(20.0)),
			requests:       []entity.APIRequest{},
			expectedOutput: "$0.0 $0.0 0%",
		},
		{
			name:           "repository error",
			formatString:   "@daily_cost",
			plan:           entity.NewPlan("pro", entity.NewCost(20.0)),
			requests:       createTestAPIRequests(3, 2, 10, 5, 5.0, 10.0, 50.0, 90.0),
			repositoryErr:  fmt.Errorf("database connection failed"),
			expectedOutput: "❌ ERROR",
			expectError:    true,
		},
		{
			name:           "all variables together",
			formatString:   "@daily_cost @monthly_cost @daily_plan_usage @monthly_plan_usage",
			plan:           entity.NewPlan("pro", entity.NewCost(20.0)),
			requests:       createTestAPIRequests(3, 2, 10, 5, 5.0, 10.0, 50.0, 90.0),
			expectedOutput: fmt.Sprintf("$15.0 $155.0 %s 775%%", calculateExpectedDailyUsage(15.0, 20.0)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock repositories using factory
			mockRepo, mockStatsRepo := testutil.NewMockRepositoryWithData(tt.requests)
			if tt.repositoryErr != nil {
				mockRepo.SetError(tt.repositoryErr)
			}

			mockPlanRepo := testutil.NewMockPlanRepository(tt.plan)
			if tt.planErr != nil {
				mockPlanRepo.SetError(tt.planErr)
			}

			// Create real services with timezone
			periodFactory := service.NewTimePeriodFactory(timezone)
			calculateStatsQuery := usecase.NewCalculateStatsQuery(mockStatsRepo, &service.NoOpStatsCache{})
			usageVariablesQuery := usecase.NewGetUsageVariablesQuery(
				calculateStatsQuery,
				mockPlanRepo,
				periodFactory,
			)

			// Create CLI components
			renderer := cli.NewFormatRenderer(usageVariablesQuery)
			queryHandler := cli.NewQueryHandler(renderer)

			// Capture output by testing the components directly
			result, err := renderer.Render(tt.formatString)

			// Verify results
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				// For error cases, test the query handler output behavior
				// In real execution, this would output "❌ ERROR"
				if !strings.Contains(tt.expectedOutput, "❌ ERROR") {
					t.Errorf("Expected error output should contain '❌ ERROR', got: %s", tt.expectedOutput)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result != tt.expectedOutput {
				t.Errorf("Expected output %q, got %q", tt.expectedOutput, result)
			}

			// Test query handler doesn't return error for successful cases
			err = queryHandler.HandleFormatQuery(tt.formatString)
			if err != nil {
				t.Errorf("QueryHandler should not return error for successful renders: %v", err)
			}
		})
	}
}

func TestTimeZoneConsistency(t *testing.T) {
	// Test that format query uses the same timezone logic as TUI
	timezones := []string{
		"UTC",
		"America/New_York",
		"Europe/London",
		"Asia/Tokyo",
		"America/Los_Angeles",
	}

	for _, tzName := range timezones {
		t.Run(fmt.Sprintf("timezone_%s", strings.ReplaceAll(tzName, "/", "_")), func(t *testing.T) {
			timezone, err := time.LoadLocation(tzName)
			if err != nil {
				t.Fatalf("Failed to load timezone %s: %v", tzName, err)
			}

			// Create requests with known timestamps in the timezone
			now := time.Now().In(timezone)
			todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timezone).UTC()
			monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, timezone).UTC()

			requests := []entity.APIRequest{
				entity.NewAPIRequest(
					"today",
					todayStart.Add(time.Hour), // Should be included in daily
					"claude-3-haiku-20240307",
					entity.NewToken(200, 160, 0, 0),
					entity.NewCost(10.0),
					1000,
				),
				entity.NewAPIRequest(
					"this-month",
					monthStart.Add(24*time.Hour), // Should be included in monthly but not daily
					"claude-3-haiku-20240307",
					entity.NewToken(200, 160, 0, 0),
					entity.NewCost(5.0),
					1000,
				),
			}

			// Setup using factory
			_, mockStatsRepo := testutil.NewMockRepositoryWithData(requests)
			mockPlanRepo := testutil.NewMockPlanRepository(entity.NewPlan("pro", entity.NewCost(20.0)))

			periodFactory := service.NewTimePeriodFactory(timezone)
			calculateStatsQuery := usecase.NewCalculateStatsQuery(mockStatsRepo, &service.NoOpStatsCache{})
			usageVariablesQuery := usecase.NewGetUsageVariablesQuery(
				calculateStatsQuery,
				mockPlanRepo,
				periodFactory,
			)

			renderer := cli.NewFormatRenderer(usageVariablesQuery)

			// Test daily cost should only include today's request
			dailyResult, err := renderer.Render("@daily_cost")
			if err != nil {
				t.Fatalf("Error rendering daily cost: %v", err)
			}
			if dailyResult != "$10.0" {
				t.Errorf("Expected daily cost $10.0, got %s", dailyResult)
			}

			// Test monthly cost should include both requests
			monthlyResult, err := renderer.Render("@monthly_cost")
			if err != nil {
				t.Fatalf("Error rendering monthly cost: %v", err)
			}
			if monthlyResult != "$15.0" {
				t.Errorf("Expected monthly cost $15.0, got %s", monthlyResult)
			}

			// Verify the period factory creates periods in the correct timezone
			dailyPeriod := periodFactory.CreateDaily()
			monthlyPeriod := periodFactory.CreateMonthly()

			// Both periods should be converted to UTC for database queries
			if dailyPeriod.StartAt().Location() != time.UTC {
				t.Errorf("Daily period start time should be in UTC")
			}
			if monthlyPeriod.StartAt().Location() != time.UTC {
				t.Errorf("Monthly period start time should be in UTC")
			}

			// Verify the periods represent the correct local time ranges in UTC
			expectedDailyStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timezone).UTC()
			if !dailyPeriod.StartAt().Equal(expectedDailyStart) {
				t.Errorf("Daily period start mismatch. Expected %v, got %v", expectedDailyStart, dailyPeriod.StartAt())
			}

			expectedMonthlyStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, timezone).UTC()
			if !monthlyPeriod.StartAt().Equal(expectedMonthlyStart) {
				t.Errorf("Monthly period start mismatch. Expected %v, got %v", expectedMonthlyStart, monthlyPeriod.StartAt())
			}
		})
	}
}

func TestVariableSubstitutionEdgeCases(t *testing.T) {
	// Setup basic test environment using factory
	timezone, _ := time.LoadLocation("America/New_York")
	_, mockStatsRepo := testutil.NewMockRepositoryWithData(createTestAPIRequests(1, 1, 5, 5, 10.0, 20.0, 50.0, 100.0))
	mockPlanRepo := testutil.NewMockPlanRepository(entity.NewPlan("pro", entity.NewCost(20.0)))

	periodFactory := service.NewTimePeriodFactory(timezone)
	calculateStatsQuery := usecase.NewCalculateStatsQuery(mockStatsRepo, &service.NoOpStatsCache{})
	usageVariablesQuery := usecase.NewGetUsageVariablesQuery(
		calculateStatsQuery,
		mockPlanRepo,
		periodFactory,
	)

	renderer := cli.NewFormatRenderer(usageVariablesQuery)

	tests := []struct {
		name           string
		formatString   string
		expectedOutput string
	}{
		{
			name:           "no variables in format string",
			formatString:   "No variables here",
			expectedOutput: "No variables here",
		},
		{
			name:           "partial variable match will substitute",
			formatString:   "prefix@daily_costsuffix",
			expectedOutput: "prefix$30.0suffix",
		},
		{
			name:           "variable at start of string",
			formatString:   "@daily_cost is today's cost",
			expectedOutput: "$30.0 is today's cost",
		},
		{
			name:           "variable at end of string",
			formatString:   "Today's cost is @daily_cost",
			expectedOutput: "Today's cost is $30.0",
		},
		{
			name:           "same variable multiple times",
			formatString:   "@daily_cost + @daily_cost = @daily_cost",
			expectedOutput: "$30.0 + $30.0 = $30.0",
		},
		{
			name:           "variables with special characters around",
			formatString:   "(@daily_cost) [@monthly_cost] {@daily_plan_usage}",
			expectedOutput: fmt.Sprintf("($30.0) [$180.0] {%s}", calculateExpectedDailyUsage(30.0, 20.0)),
		},
		{
			name:           "empty format string",
			formatString:   "",
			expectedOutput: "",
		},
		{
			name:           "only variable",
			formatString:   "@daily_cost",
			expectedOutput: "$30.0",
		},
		{
			name:           "unknown variable should not be substituted",
			formatString:   "@unknown_variable remains @unknown_variable",
			expectedOutput: "@unknown_variable remains @unknown_variable",
		},
		{
			name:           "mixed known and unknown variables",
			formatString:   "@daily_cost @unknown @monthly_cost",
			expectedOutput: "$30.0 @unknown $180.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := renderer.Render(tt.formatString)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result != tt.expectedOutput {
				t.Errorf("Expected output %q, got %q", tt.expectedOutput, result)
			}
		})
	}
}

func TestOutputFormatSpecificationCompliance(t *testing.T) {
	// Test that output formats exactly match the specification requirements
	baseRequests := createTestAPIRequests(2, 3, 10, 15, 7.5, 22.5, 75.0, 225.0)

	tests := []struct {
		name           string
		plan           entity.Plan
		formatString   string
		expectedOutput string
		description    string
		requests       []entity.APIRequest
	}{
		{
			name:           "currency format - one decimal place",
			plan:           entity.NewPlan("pro", entity.NewCost(20.0)),
			formatString:   "@daily_cost",
			expectedOutput: "$30.0",
			description:    "Currency should be formatted as USD with one decimal place",
			requests:       baseRequests,
		},
		{
			name:           "percentage format - integer",
			plan:           entity.NewPlan("pro", entity.NewCost(20.0)),
			formatString:   "@daily_plan_usage",
			expectedOutput: calculateExpectedDailyUsage(30.0, 20.0),
			description:    "Percentages should be shown as integers using new formula: daily cost / (plan price / days in current month)",
			requests:       baseRequests,
		},
		{
			name:           "percentage format - rounds down",
			plan:           entity.NewPlan("max", entity.NewCost(100.0)),
			formatString:   "@monthly_plan_usage",
			expectedOutput: "330%", // $330.0 / $100.0 = 330.0% -> 330%
			description:    "Percentages should be rounded to integers",
			requests:       baseRequests,
		},
		{
			name:           "unset plan zero percentage",
			plan:           entity.NewPlan("unset", entity.NewCost(0)),
			formatString:   "@daily_plan_usage",
			expectedOutput: "0%",
			description:    "Unset plan should always return 0% for percentages",
			requests:       baseRequests,
		},
		{
			name:           "zero cost formatting",
			plan:           entity.NewPlan("pro", entity.NewCost(20.0)),
			formatString:   "@daily_cost",
			expectedOutput: "$0.0",
			description:    "Zero costs should be formatted as $0.0",
			requests:       []entity.APIRequest{}, // No requests for zero cost
		},
		{
			name:           "large amounts formatting",
			plan:           entity.NewPlan("pro", entity.NewCost(20.0)),
			formatString:   "@monthly_cost",
			expectedOutput: "$330.0",
			description:    "Large amounts should maintain one decimal place",
			requests:       baseRequests,
		},
		{
			name:           "high percentage formatting",
			plan:           entity.NewPlan("pro", entity.NewCost(20.0)),
			formatString:   "@monthly_plan_usage",
			expectedOutput: "1650%",
			description:    "High percentages should be shown as integers",
			requests:       baseRequests,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, mockStatsRepo := testutil.NewMockRepositoryWithData(tt.requests)
			mockPlanRepo := testutil.NewMockPlanRepository(tt.plan)

			timezone, _ := time.LoadLocation("America/New_York")
			periodFactory := service.NewTimePeriodFactory(timezone)
			calculateStatsQuery := usecase.NewCalculateStatsQuery(mockStatsRepo, &service.NoOpStatsCache{})
			usageVariablesQuery := usecase.NewGetUsageVariablesQuery(
				calculateStatsQuery,
				mockPlanRepo,
				periodFactory,
			)

			renderer := cli.NewFormatRenderer(usageVariablesQuery)

			result, err := renderer.Render(tt.formatString)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result != tt.expectedOutput {
				t.Errorf("%s: Expected %q, got %q", tt.description, tt.expectedOutput, result)
			}
		})
	}
}

func TestErrorHandlingAndTimeout(t *testing.T) {
	tests := []struct {
		name          string
		repositoryErr error
		planErr       error
		expectError   bool
		description   string
	}{
		{
			name:          "repository connection error",
			repositoryErr: fmt.Errorf("connection refused"),
			expectError:   true,
			description:   "Should handle repository connection errors gracefully",
		},
		{
			name:          "repository timeout error",
			repositoryErr: fmt.Errorf("context deadline exceeded"),
			expectError:   true,
			description:   "Should handle timeout errors gracefully",
		},
		{
			name:        "plan repository error - fallback to unset",
			planErr:     fmt.Errorf("failed to read config"),
			expectError: false,
			description: "Plan repository errors should fallback to unset plan (0%)",
		},
		{
			name:        "successful execution",
			expectError: false,
			description: "Should succeed with valid data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo, mockStatsRepo := testutil.NewMockRepositoryWithData(createTestAPIRequests(1, 1, 5, 5, 10.0, 20.0, 50.0, 100.0))
			if tt.repositoryErr != nil {
				mockRepo.SetError(tt.repositoryErr)
			}

			mockPlanRepo := testutil.NewMockPlanRepository(entity.NewPlan("pro", entity.NewCost(20.0)))
			if tt.planErr != nil {
				mockPlanRepo.SetError(tt.planErr)
			}

			timezone, _ := time.LoadLocation("America/New_York")
			periodFactory := service.NewTimePeriodFactory(timezone)
			calculateStatsQuery := usecase.NewCalculateStatsQuery(mockStatsRepo, &service.NoOpStatsCache{})
			usageVariablesQuery := usecase.NewGetUsageVariablesQuery(
				calculateStatsQuery,
				mockPlanRepo,
				periodFactory,
			)

			renderer := cli.NewFormatRenderer(usageVariablesQuery)
			queryHandler := cli.NewQueryHandler(renderer)

			// Use a simple format string for testing
			formatString := "@daily_cost @daily_plan_usage"

			// For repository errors, test that renderer returns error
			_, err := renderer.Render(formatString)

			if tt.expectError {
				if err == nil {
					t.Errorf("%s: Expected error but got none", tt.description)
				}

				// Test that query handler outputs error message
				err = queryHandler.HandleFormatQuery(formatString)
				if err == nil {
					t.Errorf("%s: QueryHandler should return error for failed renders", tt.description)
				}
			} else {
				if err != nil && tt.planErr == nil {
					t.Errorf("%s: Unexpected error: %v", tt.description, err)
				}

				// For plan errors, verify fallback behavior
				if tt.planErr != nil {
					result, renderErr := renderer.Render(formatString)
					if renderErr != nil {
						t.Errorf("%s: Should not error with plan fallback: %v", tt.description, renderErr)
					}
					// Should fallback to 0% for plan usage
					if !strings.Contains(result, "0%") {
						t.Errorf("%s: Should fallback to 0%% for plan errors, got: %s", tt.description, result)
					}
				}
			}
		})
	}
}
