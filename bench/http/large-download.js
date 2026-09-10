import http from 'k6/http';
import { check } from 'k6';

export const options = {
  vus: 5,
  duration: '30s',

  thresholds: {
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  const res = http.get('https://s3.ashrix.io:8443/api/v1/download-shared-object/aHR0cDovLzEyNy4wLjAuMTo5MDAwL2VjY2xlc2l4L21lZGlhcy9jaHVyY2gxL0FsaWNlX0tpbWFuemlfRnQuX1ZpY3Rvcl9NYWVzdHJvXy1fQW1pbmlfX09mZmljaWFsX1ZpZGVvXyUyODcyMHAlMjkubXA0P1gtQW16LUFsZ29yaXRobT1BV1M0LUhNQUMtU0hBMjU2JlgtQW16LUNyZWRlbnRpYWw9V0tMQ1BRVks0QkVJT1JXOU5OTU8lMkYyMDI2MDkwOSUyRnVzLWVhc3QtMSUyRnMzJTJGYXdzNF9yZXF1ZXN0JlgtQW16LURhdGU9MjAyNjA5MDlUMjMxOTI5WiZYLUFtei1FeHBpcmVzPTQzMTk2JlgtQW16LVNlY3VyaXR5LVRva2VuPWV5SmhiR2NpT2lKSVV6VXhNaUlzSW5SNWNDSTZJa3BYVkNKOS5leUpoWTJObGMzTkxaWGtpT2lKWFMweERVRkZXU3pSQ1JVbFBVbGM1VGs1TlR5SXNJbVY0Y0NJNk1UYzRPVEF6TURrNE5Td2ljR0Z5Wlc1MElqb2lZV1J0YVc0aWZRLmJfMzVycnA4SkRyT3N4aXd0MU9Mc0tQYjBzbHBpdGZqSERWTkJGSVhTSmhjbWtya251SGRoSzE5dW5Ba2JJUExmVG1hR3RGLWNKdnowdEdJemQwYWp3JlgtQW16LVNpZ25lZEhlYWRlcnM9aG9zdCZ2ZXJzaW9uSWQ9bnVsbCZYLUFtei1TaWduYXR1cmU9MjI0M2ViZmVmOGQ2YmI0OGJmNjJjMGM5MWM3N2JlZjE2ZDFkMmUzNDRkZDIwNjk0Yzc3MjIxNzFiZDI3YjA5Ng', {
    insecureSkipTLSVerify: true,
    tags: { test: 'large-download' },
  });

  check(res, {
    'status is 200': (r) => r.status === 200,
  });
}
