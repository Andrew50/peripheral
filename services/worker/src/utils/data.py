
    def get_available_filter_values(self) -> Dict[str, List[str]]:
        """Get all available values for filter fields from the database"""
        try:
            with self.get_connection() as conn:
                cursor = conn.cursor()
                
                filter_values = {}
                
                # Get distinct sectors
                cursor.execute("""
                    SELECT DISTINCT sector 
                    FROM securities 
                    WHERE maxdate IS NULL AND active = true AND sector IS NOT NULL 
                    ORDER BY sector
                """)
                filter_values['sectors'] = [row[0] for row in cursor.fetchall()]
                
                # Get distinct industries
                cursor.execute("""
                    SELECT DISTINCT industry 
                    FROM securities 
                    WHERE maxdate IS NULL AND active = true AND industry IS NOT NULL 
                    ORDER BY industry
                """)
                filter_values['industries'] = [row[0] for row in cursor.fetchall()]
                
                # Get distinct primary exchanges
                cursor.execute("""
                    SELECT DISTINCT primary_exchange 
                    FROM securities 
                    WHERE maxdate IS NULL AND active = true AND primary_exchange IS NOT NULL 
                    ORDER BY primary_exchange
                """)
                filter_values['primary_exchanges'] = [row[0] for row in cursor.fetchall()]
                
                cursor.close()
                required_keys = ['sectors', 'industries', 'primary_exchanges']
                for key in required_keys:
                    if key not in filter_values or not filter_values[key]:
                        raise ValueError(f"Database returned empty {key} list")
                return filter_values
                
        except Exception as e:
            logger.error(f"Error fetching filter values: {e}")
            return {
                'sectors': [],
                'industries': [],
                'primary_exchanges': [],
            }