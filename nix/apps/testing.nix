# Testing-related apps: test, test-race, test-integration
{ pkgs, lib }:

{
  # Run all tests (excludes examples)
  test = {
    type = "app";
    meta.description = "Run unit tests for all modules";
    program = toString (lib.mkModuleScript {
      name = "test";
      title = "Running Tests";
      findPattern = lib.findPatterns.nonExamples;
      command = "${lib.go}/bin/go test -C ./$dir ./... -count=1";
      spinnerTitle = "Testing";
      successMsg = "All $total modules passed!";
      failMsg = "$failed/$total modules failed";
    });
  };

  # Run tests with race detector (excludes examples)
  test-race = {
    type = "app";
    meta.description = "Run tests with race detector";
    program = toString (lib.mkModuleScript {
      name = "test-race";
      title = "Running Tests with Race Detector";
      findPattern = lib.findPatterns.nonExamples;
      command = "${lib.go}/bin/go test -C ./$dir ./... -race -count=1";
      spinnerTitle = "Testing";
      successMsg = "All $total modules passed race detection!";
      failMsg = "$failed/$total modules have race conditions";
    });
  };

  # Run integration tests (excludes examples)
  test-integration = {
    type = "app";
    meta.description = "Run integration tests for all modules";
    program = toString (lib.mkModuleScript {
      name = "test-integration";
      title = "Running Integration Tests";
      findPattern = lib.findPatterns.nonExamples;
      command = "${lib.go}/bin/go test -C ./$dir ./... -tags=integration -count=1";
      spinnerTitle = "Testing";
      successMsg = "All $total modules passed integration tests!";
      failMsg = "$failed/$total modules failed integration tests";
    });
  };
}
