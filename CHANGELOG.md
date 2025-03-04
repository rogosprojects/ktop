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
- Scrollable pod list with keyboard navigation
  - Arrow keys for basic navigation
  - PgUp/PgDn for page-based scrolling
  - Home/End for jumping to first/last entries
  - Subtle row selection with arrow indicator

### Changed
- Improved UI refresh mechanism with timeout handling
- Enhanced pod panel to show sort direction indicators in column headers
- Added highlighting for the currently sorted column
- Simplified interaction model with only pod panel being interactive
- Made node and cluster summary panels view-only (non-interactive)
- Updated TAB key to always focus on the pod panel

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