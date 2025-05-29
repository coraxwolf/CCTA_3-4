package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/coraxwolf/CCTA_3-4/pkg/canvas"
	"github.com/joho/godotenv"
)

type CanvasCourse struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	WorkflowState   string `json:"workflow_state"`
	DefaultViewType string `json:"default_view"`
	Format          string `json:"course_format"`
	CourseSISID     string `json:"sis_course_id"`
}

type CanvasUser struct {
	ID    int    `json:"id" csv:"id"`
	Name  string `json:"name" csv:"name"`
	Email string `json:"email" csv:"email"`
	SisID string `json:"sis_user_id" csv:"sis_user_id"`
}

type ResultItem struct {
	CourseID        int    `json:"course_id" csv:"course_id"`
	CourseName      string `json:"course_name" csv:"course_name"`
	Subject         string `json:"subject" csv:"subject"`
	Format          string `json:"format" csv:"format"`
	WithModules     string `json:"with_modules" csv:"with_modules"`
	WithAssignments string `json:"with_assignments" csv:"with_assignments"`
	WithFrontPage   string `json:"with_front_page" csv:"with_front_page"`
	FacultyName     string `json:"faculty_name" csv:"faculty_name"`
	FacultyEmail    string `json:"faculty_email" csv:"faculty_email"`
}

var (
	api *canvas.APIManager
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		fmt.Printf("Error loading .env file: %v\n", err)
		return
	}

	logger := slog.Default()
	api = canvas.NewAPI(logger, os.Getenv("BETA_TOKEN"), os.Getenv("BETA_API_URL"), 700, 120)
	fmt.Println("Starting to fetch Summer 2025 courses...")
	// Get Summer 2025 courses (6253)
	courses, err := getCourses("6253-")
	if err != nil {
		fmt.Printf("Error fetching courses: %v\n", err)
		return
	}

	// check each course if it is part of Summer 2025
	var courseList []CanvasCourse
	for _, course := range courses {
		if strings.HasPrefix(course.CourseSISID, "6253-") {
			fmt.Printf("Course %s is part of Summer 2025\n", course.Name)
			courseList = append(courseList, course)
		}
	}

	// Pull Individual Course Data
	results := make([]ResultItem, 0) // Holder for final results
	for _, course := range courseList {
		// Check if course is published or not (workflow_state == "unavailable")
		if course.WorkflowState == "unavailable" {
			fmt.Printf("Processing course: %s (ID: %d)\n", course.Name, course.ID)
			var result ResultItem
			result.CourseID = course.ID
			result.CourseName = course.Name
			parts := strings.Split(course.CourseSISID, "-")
			if len(parts) == 4 {
				result.Subject = parts[2] // Assuming the subject is the third part of the SIS ID
			} else {
				result.Subject = "Unknown"
			}
			// Check for Modules
			mods, err := getCourseModules(course.ID)
			if err != nil {
				fmt.Printf("Error fetching modules for course %d: %v\n", course.ID, err)
				result.WithModules = "Error"
			}
			if mods {
				result.WithModules = "Yes"
			} else {
				result.WithModules = "No"
			}
			// Check if Default View is "wiki"
			if course.DefaultViewType == "wiki" {
				// Check for Front Page Content
				fp, err := getCourseFrontPage(course.ID)
				if err != nil {
					fmt.Printf("Error fetching front page for course %d: %v\n", course.ID, err)
					result.WithFrontPage = "Error"
				}
				if fp {
					result.WithFrontPage = "Yes"
				} else {
					result.WithFrontPage = "No"
				}
			}
			// Check for Assignments
			asngs, err := getCourseAssignments(course.ID)
			if err != nil {
				fmt.Printf("Error fetching assignments for course %d: %v\n", course.ID, err)
				result.WithAssignments = "Error"
			}
			if asngs {
				result.WithAssignments = "Yes"
			} else {
				result.WithAssignments = "No"
			}
			// Pull Teachers from Course
			teachers, err := getCourseTeachers(course.ID)
			if err != nil {
				fmt.Printf("Error fetching teachers for course %d: %v\n", course.ID, err)
				result.FacultyName = "Error"
				result.FacultyEmail = "Error"
			} else if len(teachers) > 0 {
				facultyNames := make([]string, 0)
				facultyEmails := make([]string, 0)
				for _, teacher := range teachers {
					facultyNames = append(facultyNames, teacher.Name)
					if teacher.Email != "" {
						facultyEmails = append(facultyEmails, teacher.Email)
					} else {
						facultyEmails = append(facultyEmails, "No Email")
					}
				}
				result.FacultyName = strings.Join(facultyNames, ", ")
				result.FacultyEmail = strings.Join(facultyEmails, ", ")
			} else {
				result.FacultyName = "No Faculty"
				result.FacultyEmail = "No Email"
			}
			results = append(results, result)
		}
	}

	// Create csv output
	filepath := path.Join("data", "reports")
	_, err = os.Stat(filepath)
	if os.IsNotExist(err) {
		err = os.MkdirAll(filepath, 0755)
		if err != nil {
			fmt.Printf("Error creating directory %s: %v\n", filepath, err)
			return
		}
	}
	outputFile := path.Join(filepath, "summer_2025_unpublished_courses.csv")
	of, err := os.OpenFile(outputFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Printf("Error opening output file %s: %v\n", outputFile, err)
		return
	}
	defer of.Close()
	writer := csv.NewWriter(of)
	defer writer.Flush()
	header := []string{"course_id", "course_name", "subject", "with_modules", "with_assignments", "with_front_page", "faculty_name", "faculty_email"}
	if err := writer.Write(header); err != nil {
		fmt.Printf("Error writing header to CSV: %v\n", err)
		return
	}
	fmt.Printf("Written Report to %s with %d entries\n", outputFile, len(results))
}

func getCourses(search string) ([]CanvasCourse, error) {
	ep := fmt.Sprintf("accounts/1/courses?search_term=%s&per_page=100", search)
	next := true               // assume more than one page of results
	var courses []CanvasCourse // holder for all courses
	page := 1                  // page counter for debugging
	for next {
		resp, err := api.Get(ep)
		if err != nil {
			return nil, fmt.Errorf("error fetching courses: %w", err)
		}
		defer resp.Body.Close() // Ensure the response body is closed after reading
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("error fetching courses: received status code %d", resp.StatusCode)
		}
		var pageCourses []CanvasCourse
		if err := json.NewDecoder(resp.Body).Decode(&pageCourses); err != nil {
			return nil, fmt.Errorf("error decoding courses response: %w", err)
		}
		courses = append(courses, pageCourses...) // Add found courses to the final list
		// Check if there is a next page
		headers := resp.Header.Get("Link")
		if headers == "" {
			fmt.Printf("WARNING: No Link header found, assuming no more pages\n")
			next = false
		} else {
			parts := strings.Split(headers, ",")
			for _, part := range parts {
				if strings.Contains(part, `rel="next"`) {
					// Extract the URL for the next page
					link := strings.Split(part, ";")
					link[0] = strings.Trim(link[0], " <>")
					ep = link[0] // Update the endpoint to the next page
					next = true
					continue // Break out of the loop to fetch the next page
				}
				fmt.Printf("WARNING: No next page found in Link header, stopping pagination\n")
				next = false // No next page found, stop pagination
			}
		}
		page++
		fmt.Printf("Getting Page %d of courses\n", page)
	}
	return courses, nil
}

func getCourseTeachers(courseID int) ([]CanvasUser, error) {
	fac, err := api.Get(fmt.Sprintf("courses/%d/teachers?per_page=100", courseID))
	if err != nil {
		return nil, fmt.Errorf("error fetching teachers for course %d: %w", courseID, err)
	}
	if fac.StatusCode != 200 {
		return nil, fmt.Errorf("error fetching teachers for course %d: received status code %d", courseID, fac.StatusCode)
	}
	var teachers []CanvasUser
	if err := json.NewDecoder(fac.Body).Decode(&teachers); err != nil {
		return nil, fmt.Errorf("error decoding teachers response for course %d: %w", courseID, err)
	}

	return teachers, nil
}

func getCourseModules(courseID int) (bool, error) {
	resp, err := api.Get(fmt.Sprintf("courses/%d/modules", courseID))
	if err != nil {
		return false, fmt.Errorf("error fetching course %d: %w", courseID, err)
	}
	if resp.StatusCode != 200 {
		return false, fmt.Errorf("error fetching course %d: received status code %d", courseID, resp.StatusCode)
	}
	defer resp.Body.Close() // Ensure the response body is closed after reading
	var mods []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&mods); err != nil {
		return false, fmt.Errorf("error decoding modules response for course %d: %w", courseID, err)
	}
	if len(mods) > 0 {
		return true, nil // Course has modules
	}
	return false, nil
}

func getCourseAssignments(courseID int) (bool, error) {
	resp, err := api.Get(fmt.Sprintf("courses/%d/assignments?per_page=100", courseID))
	if err != nil {
		return false, fmt.Errorf("error fetching assignments for course %d: %w", courseID, err)
	}
	defer resp.Body.Close() // Ensure the response body is closed after reading
	if resp.StatusCode != 200 {
		return false, fmt.Errorf("error fetching assignments for course %d: received status code %d", courseID, resp.StatusCode)
	}
	var assignments []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&assignments); err != nil {
		return false, fmt.Errorf("error decoding assignments response for course %d: %w", courseID, err)
	}
	if len(assignments) > 0 {
		return true, nil // Course has assignments
	}

	return false, nil
}

func getCourseFrontPage(courseID int) (bool, error) {
	resp, err := api.Get(fmt.Sprintf("courses/%d/front_page", courseID))
	if err != nil {
		return false, fmt.Errorf("error fetching front page for course %d: %w", courseID, err)
	}
	defer resp.Body.Close() // Ensure the response body is closed after reading
	if resp.StatusCode != 200 {
		return false, fmt.Errorf("error fetching front page for course %d: received status code %d", courseID, resp.StatusCode)
	}
	var frontPage map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&frontPage); err != nil {
		return false, fmt.Errorf("error decoding front page response for course %d: %w", courseID, err)
	}
	if content, ok := frontPage["body"].(string); ok && content != "" {
		return true, nil // Course has front page content
	}
	return false, nil
}
