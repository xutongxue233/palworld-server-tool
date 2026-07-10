import { useFetch } from "@vueuse/core";

class Service {
  /**
   * Fetches data from a specified URL.
   *
   * @param {string} url - The URL to fetch data from.
   * @return {Promise<Response>} A Promise that resolves to the response from the server.
   */
  fetch(url) {
    return useFetch(`${url}`, {
      updateDataOnError: true,
      beforeFetch({ options }) {
        const token = localStorage.getItem("palworld_token");
        options.headers = {
          "Content-Type": "application/json",
          ...options.headers,
        };
        if (token) options.headers.Authorization = `Bearer ${token}`;
        return {
          options,
        };
      },
      onFetchError(context) {
        if (context.response?.status === 401) {
          localStorage.removeItem("palworld_token");
        }
        return context;
      },
    });
  }

  /**
   * Generates a query string from a given credential object.
   *
   * @param {Object} credential - The credential object.
   * @return {string} - The generated query string.
   */
  generateQuery(credential = {}) {
    const query = new URLSearchParams();
    Object.entries(credential).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== "") {
        query.append(key, String(value));
      }
    });
    return query.toString();
  }
}

export default Service;
