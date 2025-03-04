# Changelog

All notable changes to the ktop project will be documented in this file.

## [Unreleased] - 2025-03-04

### Added
- Column sorting for pod lists
  - Support for sorting by any column (namespace, pod name, CPU, memory, etc.)
  - Both Shift+key and uppercase letter support for better terminal compatibility
  - Visual indicators showing current sort column and direction
  - Toggle between ascending/descending order by pressing same key twice
- Footer help text showing available sort keys

### Changed
- Improved UI refresh mechanism with timeout handling
- Enhanced pod panel to show sort direction indicators in column headers
- Added highlighting for the currently sorted column

## [Previous Releases]

### [2025-02-29]
- Refactor layout structure in main panel to improve node and pod view organization

### [2025-02-27]
- Add peak resource tracking for nodes and pods in summary controller

### [2025-02-25]
- Add customizable refresh intervals for ktop summary, nodes, and pods

### [2025-02-20]
- Merge bugfix/memory-graph into main
- Fix display of memory usage showing as a percentage of the limit instead of requested memory