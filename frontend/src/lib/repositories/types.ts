export type Repository = {
	id: number;
	owner: string;
	name: string;
	full_name: string;
	html_url: string;
	description: string;
	private: boolean;
	default_branch: string;
	selected: boolean;
};

export type RepositoryList = {
	repositories: Repository[];
};
