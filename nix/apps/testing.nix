# Testing-related apps (run outside sandbox with network access)
{ pkgs, lib }:

{
  # Run all tests (excludes examples) - no coverage
  test = {
    type = "app";
    meta.description = "Run unit tests for all modules";
    program = toString (lib.mkModuleScript {
      name = "test";
      title = "Running Tests";
      findPattern = lib.findPatterns.nonExamples;
      command = "${pkgs.go}/bin/go test -C ./$dir ./... -count=1";
      spinnerTitle = "Testing";
      successMsg = "All $total modules passed!";
      failMsg = "$failed/$total modules failed";
    });
  };

  # Run tests with race detector and generate coverage report
  test-race = {
    type = "app";
    meta.description = "Run tests with race detector and generate coverage";
    program = toString (lib.mkCoverageTestScript {
      name = "test-race";
      title = "Running Tests with Race Detector";
      testFlags = "-race";
      coverageDir = ".coverage";
      outputFile = "coverage.out";
      spinnerTitle = "Testing with race detector";
      successMsg = "All $total modules passed race detection!";
      failMsg = "$failed/$total modules have race conditions";
    });
  };

  # Run integration tests and generate coverage report
  test-integration = {
    type = "app";
    meta.description = "Run integration tests and generate coverage";
    program = toString (lib.mkCoverageTestScript {
      name = "test-integration";
      title = "Running Integration Tests";
      testFlags = "-tags=integration";
      coverageDir = ".coverage-integration";
      outputFile = "coverage-integration.out";
      spinnerTitle = "Testing";
      successMsg = "All $total modules passed integration tests!";
      failMsg = "$failed/$total modules failed integration tests";
    });
  };
}
