./ft-cv-puller -role 5000154941  -out ./cvs --debug 
  curl -sS -o /dev/null -w 'applicant fields: %{http_code}\n' \
    -H "Authorization: Bearer $TOKEN" \
    https://quantifisolutions.freshteam.com/api/job_postings/5000154941/applicant_fields


 TOKEN=$(cat ~/.ft-cv-puller)

  # 3a. List all published job postings (Recruit module, read)
  curl -sS -w '\nstatus: %{http_code}\n' -H "Authorization: Bearer $TOKEN" 'https://quantifisolutions.freshteam.com/api/job_postings?status=published' | head -80

  # 3b. List ALL job postings (no filter) — even broader Recruit read
  curl -sS -w '\nstatus: %{http_code}\n' -H "Authorization: Bearer $TOKEN" 'https://quantifisolutions.freshteam.com/api/job_postings' | head -80

  # 3c. Sanity: hit a non-Recruit (HR) endpoint to compare scopes
  curl -sS -o /dev/null -w 'employees: %{http_code}\n' -H "Authorization: Bearer $TOKEN" 'https://quantifisolutions.freshteam.com/api/employees?page=1'