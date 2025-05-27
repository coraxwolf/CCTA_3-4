# CCTA_3-4

Assignment is to build compound API Calls to achieve a desired result.

## Result

Every Course in the next term should be published by the first day of term.
This will be a check ran the Friday beforer the start of term to Alert any Faculty that does not have their course published.

## API Calls

This will include an iterative request and a looped request.

### Get Endpoints

- Get all courses for a term -- GET /api/v1/accounts/1/courses?search_term=6253
- Get Course Details -- GET /api/v1/courses/{{course_id}}
- Get Course Front Page -- GET /api/v1/courses/{{course_id}}/front_page
- Get Course Assignments -- GET /api/v1/courses/{{course_id}}/assignments?published=true
- Get Course Modules -- GET /api/v1/courses/{{course_id}}/modules?published=true
